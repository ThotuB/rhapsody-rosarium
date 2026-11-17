"""The transcriber: the piano notes in a WAV recording, for the projector's keyboard (Python 3.10).

The museum app posts each segment it's about to play to POST /transcribe and gets its notes back as
json, see README.md. Run with `uv run serve.py`. Basic Pitch's model is loaded once, at startup; each
recording is written to a temporary folder for it and deleted right after.
"""

import io
import json
import logging
import math
import tempfile
import threading
import time
from collections.abc import Iterable
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import TYPE_CHECKING, TypedDict, cast

import soundfile

if TYPE_CHECKING:  # importing Basic Pitch takes seconds, main() does it after saying so
    from basic_pitch.inference import Model

ADDRESS = ("127.0.0.1", 8765)  # only this laptop: the museum app runs next to it
MAX_BYTES = 32 * 1024 * 1024
MAX_SECONDS = 180

# the piano's 88 keys as MIDI notes, A0 to C8
LOWEST, HIGHEST = 21, 108
# what Basic Pitch listens for: from A0's frequency to C8's, notes of at least 80 ms
MIN_HZ, MAX_HZ = 27.5, 4187.0
MIN_NOTE_MS = 80.0


class Note(TypedDict):
    pitch: int  # MIDI note, LOWEST to HIGHEST
    start: float  # seconds into the recording
    duration: float  # seconds
    velocity: int  # how hard it was played, 0 to 127


class Transcription(TypedDict):
    notes: list[Note]  # by start
    duration: float  # of the recording, seconds
    analysisSeconds: float  # how long Basic Pitch took


class Rejected(Exception):
    """A request that can't be transcribed, answered with its HTTP status and the message."""

    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status


def normalize_notes(events: Iterable[tuple], duration: float) -> list[Note]:
    """Basic Pitch's note events as Notes: only the piano's keys, cut to the recording, by start.

    Its amplitude, 0 to 1, becomes a MIDI style velocity, 0 to 127.
    """
    notes: list[Note] = []
    for start, end, pitch, amplitude, *_ in events:
        start, end = max(0.0, float(start)), min(duration, float(end))
        pitch, amplitude = int(pitch), float(amplitude)
        if not all(math.isfinite(v) for v in (start, end, amplitude)):
            continue
        if LOWEST <= pitch <= HIGHEST and end > start:
            velocity = max(0, min(127, round(amplitude * 127)))
            notes.append(
                {
                    "pitch": pitch,
                    "start": start,
                    "duration": end - start,
                    "velocity": velocity,
                }
            )
    return sorted(notes, key=lambda note: note["start"])


def transcribe(model: "Model", wav: bytes) -> Transcription:
    """The notes in a WAV recording. Raises Rejected if it isn't one."""
    if len(wav) < 12 or wav[:4] not in (b"RIFF", b"RF64") or wav[8:12] != b"WAVE":
        raise Rejected(400, "Not a WAV recording.")
    try:
        duration = float(soundfile.info(io.BytesIO(wav)).duration)
    except soundfile.SoundFileError:
        raise Rejected(400, "Can't decode this WAV, 16-bit PCM works.") from None
    if not math.isfinite(duration) or not 0 < duration <= MAX_SECONDS:
        raise Rejected(400, f"Recordings can be up to {MAX_SECONDS} s long.")

    from basic_pitch.inference import predict  # imported by main() already, so instant

    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="rosarium-notes-") as directory:
        path = Path(directory) / "segment.wav"  # Basic Pitch only reads files
        path.write_bytes(wav)
        _, _, events = predict(
            str(path),
            model,
            minimum_frequency=MIN_HZ,
            maximum_frequency=MAX_HZ,
            minimum_note_length=MIN_NOTE_MS,
        )
    return {
        "notes": normalize_notes(events, duration),
        "duration": duration,
        "analysisSeconds": round(time.monotonic() - started, 3),
    }


class Server(ThreadingHTTPServer):
    """The transcriber's HTTP server, with Basic Pitch's model for every request."""

    daemon_threads = True

    def __init__(self, model: "Model"):
        super().__init__(ADDRESS, Handler)
        self.model = model
        # one recording at a time; the museum sends them one by one anyway
        self.busy = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    """GET /health and POST /transcribe."""

    timeout = 60  # seconds a connection may stall, e.g. halfway through an upload

    @property
    def app(self) -> Server:
        return cast(Server, self.server)  # always a Server, see main()

    def do_GET(self):
        if self.path != "/health":
            self.reply(404, {"error": "Unknown endpoint."})
            return
        self.reply(
            200, {"ready": True, "model": "Basic Pitch", "busy": self.app.busy.locked()}
        )

    def do_POST(self):
        if self.path != "/transcribe":
            self.reply(404, {"error": "Unknown endpoint."})
            return
        if not self.app.busy.acquire(blocking=False):
            self.reply(409, {"error": "Busy with another recording, retry after it."})
            return
        try:
            self.reply(200, transcribe(self.app.model, self.read_upload()))
        except Rejected as e:
            self.reply(e.status, {"error": str(e)})
        except (TimeoutError, ConnectionError):
            logging.warning("Upload closed or timed out")
        except Exception:
            logging.exception("Transcription failed")
            self.reply(
                500, {"error": "Transcription failed, see the transcriber's log."}
            )
        finally:
            self.app.busy.release()

    def read_upload(self) -> bytes:
        try:
            size = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            raise Rejected(400, "Invalid Content-Length.") from None
        if not 0 < size <= MAX_BYTES:
            raise Rejected(413, f"Uploads can be up to {MAX_BYTES >> 20} MB.")
        audio = self.rfile.read(size)
        if len(audio) != size:
            raise Rejected(400, "The upload was incomplete.")
        return audio

    def reply(self, status: int, data: object):
        body = json.dumps(data, allow_nan=False).encode()
        self.send_response(status)
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError):
            pass  # the museum stopped waiting: it timed out, or was tuned to another piano


def main():
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    print(
        "Loading Basic Pitch. The first start can take a while, the libraries warm up.",
        flush=True,
    )
    from basic_pitch import ICASSP_2022_MODEL_PATH
    from basic_pitch.inference import Model

    with Server(Model(ICASSP_2022_MODEL_PATH)) as server:
        host, port = ADDRESS
        print(f"Ready at http://{host}:{port}, for the museum app.", flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            print("\nStopping the transcriber.", flush=True)


if __name__ == "__main__":
    main()
