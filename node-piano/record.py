"""Continuous microphone recording into numbered MP3 segment files.

FFmpeg writes one file per SEGMENT_SECONDS into DIRECTORY, named by its
sequence number; app.py uploads and removes them.
"""

import json
import signal
import subprocess
import threading
import time
from pathlib import Path


# Configuration
MIC = "plughw:CARD=Microphones,DEV=0"
DIRECTORY = Path(__file__).parent / "segments"
STATE = DIRECTORY / "state.json"

SEGMENT_SECONDS = 2

# Loudness: the mic's level is turned up by GAIN_DB, and a limiter keeps the loudest notes just
# under full scale (LIMIT, about -1 dB) instead of clipping. Measured on piano-2: the recording
# averaged -19 dB with peaks at -6 dB; +8 dB averages -12 dB, +14 dB about -9 dB. More than
# that hardly gets louder (+18 dB: -8.4 dB), the limiter only squashes the music harder
GAIN_DB = 14
LIMIT = 0.89

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
# Continuous recording
# ---------------------------

def record_forever(stop, active, gain_db=GAIN_DB):
    """Record until stop is set; active is set while FFmpeg writes a segment.

    gain_db turns the recording up (or down, if negative) before the limiter.
    """
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

            # Louder, without clipping (level=0: the limiter only holds peaks down,
            # it doesn't turn everything up again on its own)
            "-af", f"volume={gain_db}dB,alimiter=limit={LIMIT}:level=0",

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
