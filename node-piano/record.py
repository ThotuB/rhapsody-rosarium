
"""Continuous microphone recording with persistent MP3 uploads."""

import argparse
import json
import signal
import subprocess
import threading
import time
from pathlib import Path

import requests
import api


# Configuration
MIC = "plughw:CARD=Microphones,DEV=0"
DIRECTORY = Path(__file__).parent / "segments"
STATE = DIRECTORY / "state.json"

SEGMENT_SECONDS = 2
RETRY_SECONDS = 3

MAX_PENDING_SEGMENTS = 3
MAX_SEGMENT_AGE = 8

# Prevent concurrent state-file operations
STATE_LOCK = threading.Lock()


# ---------------------------
# Sequence management
# ---------------------------

def _read_state():
    if STATE.exists():
        return int(
            json.loads(STATE.read_text()).get("next_seq", 0)
        )
    return 0


def save_next_seq(value):
    """Atomically save a monotonically increasing sequence number."""
    with STATE_LOCK:
        current = _read_state()
        value = max(current, value)

        tmp = STATE.with_suffix(".tmp")
        tmp.write_text(json.dumps({"next_seq": value}))
        tmp.replace(STATE)


def next_sequence():
    """Determine the next available segment number."""
    with STATE_LOCK:
        previous = _read_state()

        files = list(DIRECTORY.glob("*.mp3"))

        if files:
            previous = max(
                previous,
                max(int(p.stem) for p in files) + 1
            )

        return previous


def segment_files():
    """Return pending recordings in sequence order."""
    return sorted(
        DIRECTORY.glob("*.mp3"),
        key=lambda p: int(p.stem)
    )


# ---------------------------
# Heartbeat
# ---------------------------

def heartbeat(stop, args):
    session = requests.Session()

    while not stop.is_set():
        try:
            response = api.post_heartbeat(
                session,
                args.server,
                args.id,
                args.token
            )

            if not response.ok:
                print(
                    f"Heartbeat error: HTTP {response.status_code}",
                    flush=True
                )

        except requests.RequestException as exc:
            print(
                f"Heartbeat unavailable: {exc}",
                flush=True
            )

        stop.wait(10)


# ---------------------------
# Upload worker
# ---------------------------

def uploader(stop, args, active):
    session = requests.Session()

    while not stop.is_set():
        files = segment_files()

        # Never upload the file currently being written.
        if active.is_set() and files:
            completed = files[:-1]
        else:
            completed = files

        now = time.time()

        # Keep only recent, completed segments.
        recent = []
        for path in completed:
            try:
                if now - path.stat().st_mtime <= MAX_SEGMENT_AGE:
                    recent.append(path)
            except FileNotFoundError:
                continue

        recent = recent[-MAX_PENDING_SEGMENTS:]
        keep = set(recent)

        # Remove old or excess completed recordings.
        for path in completed:
            if path not in keep:
                try:
                    path.unlink()
                    print(
                        f"Dropped old segment: {path.name}",
                        flush=True
                    )
                except FileNotFoundError:
                    pass

        if not recent:
            stop.wait(0.2)
            continue

        # Upload recent segments in order.
        for path in recent:
            if stop.is_set():
                return

            try:
                # Check freshness again before uploading.
                age = time.time() - path.stat().st_mtime

                if age > MAX_SEGMENT_AGE:
                    path.unlink()
                    print(
                        f"Skipped stale segment: {path.name}",
                        flush=True
                    )
                    continue

                seq = int(path.stem)
                save_next_seq(seq + 1)

                response = api.post_segment(
                    session,
                    args.server,
                    args.id,
                    args.token,
                    path.name,
                    path.read_bytes(),
                    seq,
                )

                if response.ok:
                    print(
                        f"Uploaded segment {seq}: "
                        f"HTTP {response.status_code}",
                        flush=True
                    )
                    path.unlink()

                elif response.status_code == 422:
                    print(
                        f"Invalid audio, discarding {path.name}",
                        flush=True
                    )
                    path.unlink()

                else:
                    print(
                        f"Upload failed: HTTP "
                        f"{response.status_code}",
                        flush=True
                    )
                    stop.wait(RETRY_SECONDS)
                    break

            except FileNotFoundError:
                continue

            except (requests.RequestException, OSError) as exc:
                print(
                    f"Connection unavailable: {exc}",
                    flush=True
                )
                stop.wait(RETRY_SECONDS)
                break

# ---------------------------
# Continuous recording
# ---------------------------

def record_forever(stop, active):
    while not stop.is_set():

        seq = next_sequence()
        save_next_seq(seq)

        print(
            f"Starting FFmpeg at segment {seq}",
            flush=True
        )

        command = [
            "ffmpeg",
            "-nostdin",
            "-hide_banner",
            "-loglevel", "warning",

            # Microphone
            "-f", "alsa",
            "-i", MIC,

            # MP3 audio encoding
            "-ac", "1",
            "-ar", "44100",
            "-c:a", "libmp3lame",
            "-b:a", "128k",

            # Continuous segmentation
            "-f", "segment",
            "-segment_time", str(SEGMENT_SECONDS),
            "-segment_start_number", str(seq),
            "-reset_timestamps", "1",

            str(DIRECTORY / "%012d.mp3"),
        ]

        active.set()
        process = None

        try:
            process = subprocess.Popen(command)

            while process.poll() is None and not stop.is_set():
                time.sleep(0.5)

        except KeyboardInterrupt:
            stop.set()

        except OSError as exc:
            print(f"FFmpeg failed to start: {exc}", flush=True)

        finally:
            if process is not None:
                if process.poll() is None:
                    process.send_signal(signal.SIGINT)

                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()

                print(
                    f"FFmpeg exited: {process.returncode}",
                    flush=True
                )

            active.clear()

            # Preserve the next available number.
            save_next_seq(next_sequence())

        if not stop.is_set():
            print(
                "Recording interrupted. Restarting in 3 seconds...",
                flush=True
            )
            stop.wait(3)


# ---------------------------
# Main
# ---------------------------

def main():
    parser = argparse.ArgumentParser()

    parser.add_argument("--id", required=True)
    parser.add_argument("--server", required=True)
    parser.add_argument("--token", required=True)

    args = parser.parse_args()
    args.server = args.server.rstrip("/")

    DIRECTORY.mkdir(parents=True, exist_ok=True)

    stop = threading.Event()
    active = threading.Event()

    # Start heartbeat worker
    heartbeat_thread = threading.Thread(
        target=heartbeat,
        args=(stop, args),
        daemon=True
    )
    heartbeat_thread.start()

    # Start upload worker
    upload_thread = threading.Thread(
        target=uploader,
        args=(stop, args, active),
        daemon=True
    )
    upload_thread.start()

    print(
        "Starting continuous microphone capture...",
        flush=True
    )

    try:
        record_forever(stop, active)

    except KeyboardInterrupt:
        stop.set()

    finally:
        stop.set()
        print(
            "Recorder stopped. Pending MP3 files remain on disk.",
            flush=True
        )


if __name__ == "__main__":
    main()
