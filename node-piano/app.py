"""The piano node: records the microphone, uploads the segments and shows its listeners.

record.py writes numbered segment files; this uploads them in order and sends
a heartbeat, whose reply says how many people listen to this piano, which sets
how fast the LED pulses, and the colour the museum chose for it (see lights.py).

    uv run app.py --id piano-2 --server http://10.69.251.6:8080/api --token dev
"""

import argparse
import threading
import time

import requests

import api
import lights
import record

# Configuration
RETRY_SECONDS = 3
HEARTBEAT_SECONDS = 2  # Also how soon the LED reacts to listeners

MAX_PENDING_SEGMENTS = 3
MAX_SEGMENT_AGE = 8
# Seconds to connect, and to wait for the answer, per upload: well under MAX_SEGMENT_AGE, so a
# stuck upload (a wifi hiccup) is given up and retried before the segments behind it go stale
UPLOAD_TIMEOUT = (3, 6)


# ---------------------------
# Heartbeat
# ---------------------------


def heartbeat(stop, args, light):
    """Tell the server the node is online, and show its listeners on the LED,
    in the colour the museum set."""
    session = requests.Session()
    heard = None  # Last listeners and colour, to print only changes

    while not stop.is_set():
        listeners = 0  # Fade out on connection loss
        colour = None  # Keep the last colour on connection loss

        try:
            response = api.post_heartbeat(session, args.server, args.id, args.token)

            if response.ok:
                # {"listeners": 2, "museum": true, "led": "#ff00ff"}, listeners counts the museum too
                data = response.json()
                listeners = int(data["listeners"])
                museum = bool(data.get("museum", False))
                colour = data.get("led")

                if (listeners, colour) != heard:
                    print(
                        f"Listeners: {listeners} | Museum: {museum} | LED: {colour}",
                        flush=True,
                    )
                    heard = (listeners, colour)

            else:
                print(f"Heartbeat error: HTTP {response.status_code}", flush=True)

        # RequestException includes invalid JSON
        except (requests.RequestException, KeyError, TypeError, ValueError) as exc:
            print(f"Heartbeat unavailable: {exc}", flush=True)

        if light is not None:
            light.show(listeners, colour)

        stop.wait(HEARTBEAT_SECONDS)


# ---------------------------
# Upload worker
# ---------------------------


def uploader(stop, args, active):
    session = requests.Session()

    while not stop.is_set():
        files = record.segment_files()

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
                    print(f"Dropped old segment: {path.name}", flush=True)
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
                    print(f"Skipped stale segment: {path.name}", flush=True)
                    continue

                seq = int(path.stem)
                record.save_next_seq(seq + 1)

                response = api.post_segment(
                    session,
                    args.server,
                    args.id,
                    args.token,
                    path.name,
                    path.read_bytes(),
                    seq,
                    timeout=UPLOAD_TIMEOUT,
                )

                if response.ok:
                    print(
                        f"Uploaded segment {seq}: " f"HTTP {response.status_code}",
                        flush=True,
                    )
                    path.unlink()

                elif response.status_code == 422:
                    print(f"Invalid audio, discarding {path.name}", flush=True)
                    path.unlink()

                else:
                    print(f"Upload failed: HTTP " f"{response.status_code}", flush=True)
                    stop.wait(RETRY_SECONDS)
                    break

            except FileNotFoundError:
                continue

            except (requests.RequestException, OSError) as exc:
                print(f"Connection unavailable: {exc}", flush=True)
                stop.wait(RETRY_SECONDS)
                break


# ---------------------------
# Main
# ---------------------------


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument("--id", required=True)
    parser.add_argument("--server", required=True)
    parser.add_argument("--token", required=True)
    parser.add_argument(
        "--gain-db",
        type=float,
        default=record.GAIN_DB,
        help=f"turn the recording up by this many dB, peaks held under -1 dB (default {record.GAIN_DB})",
    )

    args = parser.parse_args()
    args.server = args.server.rstrip("/")

    record.DIRECTORY.mkdir(parents=True, exist_ok=True)

    stop = threading.Event()
    active = threading.Event()

    # Start LED worker, if the LED is there
    light = lights.open_light()

    if light is not None:
        light.start(stop)

    # Start heartbeat worker
    heartbeat_thread = threading.Thread(
        target=heartbeat, args=(stop, args, light), daemon=True
    )
    heartbeat_thread.start()

    # Start upload worker
    upload_thread = threading.Thread(
        target=uploader, args=(stop, args, active), daemon=True
    )
    upload_thread.start()

    print("Starting continuous microphone capture...", flush=True)

    try:
        record.record_forever(stop, active, args.gain_db)

    except KeyboardInterrupt:
        stop.set()

    finally:
        stop.set()

        if light is not None:
            light.close()

        print("Node stopped. Pending MP3 files remain on disk.", flush=True)


if __name__ == "__main__":
    main()
