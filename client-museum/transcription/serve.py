"""Local WAV-to-note helper for the projector prototype (Python 3.10).

Run after installing requirements.txt. Files live in a temporary directory and
are removed after inference. The model is loaded once and reused.
"""

import json
import logging
import math
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import tempfile
import threading
import time

MAX_BYTES = 32 * 1024 * 1024
MAX_SECONDS = 180
MODEL_LOCK = threading.Lock()


def normalize_notes(events, duration):
    """Convert Basic Pitch's 0..1 amplitude to a MIDI-style 0..127 velocity."""
    notes = []
    for start, end, pitch, velocity, *_ in events:
        start, end = max(0.0, float(start)), min(duration, float(end))
        pitch, velocity = int(pitch), float(velocity)
        if not all(math.isfinite(v) for v in (start, end, velocity)):
            continue
        if 21 <= pitch <= 108 and end > start:
            notes.append({
                "pitch": pitch, "start": start, "duration": end - start,
                "velocity": max(0, min(127, round(velocity * 127))),
            })
    return sorted(notes, key=lambda note: note["start"])


class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(60)

    def reply(self, status, data):
        body = json.dumps(data, allow_nan=False).encode("utf-8")
        self.send_response(status)
        # The helper is loopback-only; Wails and Vite use different origins.
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.send_header("Access-Control-Allow-Private-Network", "true")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError):
            pass  # Window closed or a different file was selected.

    def do_OPTIONS(self):
        self.reply(200, {})

    def do_GET(self):
        if self.path == "/health":
            self.reply(200, {"ready": True, "model": "Basic Pitch", "busy": MODEL_LOCK.locked()})
        else:
            self.reply(404, {"error": "Unknown endpoint."})

    def do_POST(self):
        if self.path != "/transcribe":
            self.reply(404, {"error": "Unknown endpoint."})
            return
        try:
            size = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self.reply(400, {"error": "Invalid upload length."})
            return
        if not 0 < size <= MAX_BYTES:
            self.reply(413, {"error": "Choose a nonempty WAV smaller than 32 MB."})
            return
        if not MODEL_LOCK.acquire(blocking=False):
            self.reply(409, {"error": "Another recording is being analyzed. Wait for it to finish, then retry."})
            return
        try:
            audio = self.rfile.read(size)
            if len(audio) != size:
                self.reply(400, {"error": "The audio upload was incomplete."})
                return
            if len(audio) < 12 or audio[:4] not in (b"RIFF", b"RF64") or audio[8:12] != b"WAVE":
                self.reply(400, {"error": "This file is not a WAV recording."})
                return
            with tempfile.TemporaryDirectory(prefix="rosarium-notes-") as directory:
                path = Path(directory) / "recording.wav"
                path.write_bytes(audio)
                try:
                    import soundfile as sf
                    info = sf.info(str(path))
                except Exception:
                    self.reply(400, {"error": "Cannot decode this WAV. Export it as 16-bit PCM WAV."})
                    return
                duration = float(info.duration)
                if not math.isfinite(duration) or not 0 < duration <= MAX_SECONDS:
                    self.reply(400, {"error": "Choose a recording between 0 and 180 seconds long."})
                    return
                started = time.monotonic()
                _, _, events = self.server.predict(str(path), self.server.model,
                    minimum_frequency=27.5, maximum_frequency=4187.0,
                    minimum_note_length=80.0)
                self.reply(200, {"notes": normalize_notes(events, duration), "duration": duration,
                                 "analysisSeconds": round(time.monotonic() - started, 3)})
        except (TimeoutError, ConnectionError):
            logging.warning("Upload connection closed or timed out")
        except Exception:
            logging.exception("Transcription failed")
            self.reply(500, {"error": "Note recognition failed. See the Python helper terminal for details."})
        finally:
            MODEL_LOCK.release()


def main():
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    print("Loading Basic Pitch. The first analysis may take longer while libraries warm up.", flush=True)
    from basic_pitch import ICASSP_2022_MODEL_PATH
    from basic_pitch.inference import Model, predict
    model = Model(ICASSP_2022_MODEL_PATH)
    server = ThreadingHTTPServer(("127.0.0.1", 8765), Handler)
    server.daemon_threads = True
    server.model, server.predict = model, predict
    print("Ready at http://127.0.0.1:8765 — choose a WAV in the projector window.", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping local transcription helper.", flush=True)
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
