# transcriber

A small local helper for the museum: it listens to each segment the museum is about to play and says which piano
keys were pressed, so the projector's keyboard lights them up in time with the speakers. It runs
[Basic Pitch](https://github.com/spotify/basic-pitch), Spotify's audio to notes model, which comes with the
package: no API key, and the audio never leaves the laptop.

It's optional. Without it the museum plays as usual and the keyboard stays unlit, with "Note recognition
unavailable" on the projector.

## Set up once

Python 3.10 only, see `pyproject.toml`. The exact version of every package is in `uv.lock`.

With [uv](https://docs.astral.sh/uv/), on Linux or Windows, which also fetches Python 3.10:

```sh
cd client-museum/transcriber
uv sync
```

Or on Windows without uv, with 64-bit Python 3.10 from https://www.python.org/downloads/release/python-31011/
(`py -3.10 --version`). `requirements.txt` is the same versions as `uv.lock`, for pip:

```powershell
cd client-museum\transcriber
py -3.10 -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
```

No activating the venv, so PowerShell's script execution policy doesn't get in the way.

After changing the dependencies in `pyproject.toml`: `uv lock`, then
`uv export --format requirements-txt --no-hashes --no-emit-project -o requirements.txt` to update the pip list too.

## Run

Next to the museum app, in its own terminal:

```sh
uv run serve.py                          # with uv
.\.venv\Scripts\python.exe serve.py      # Windows, without uv
```

It's ready at **http://127.0.0.1:8765** after a few seconds; the very first start compiles parts of the libraries
and can take much longer. On Linux it runs the TFLite model, and the "Tensorflow is not installed" warning is
harmless. The first segment takes ~3 s while the model warms up, then each 2 s segment ~0.3 s.

Only one runs at a time: close the old one if the port is taken.

```sh
uv run python -m unittest test_serve.py    # the note conversion, no model needed
```

## How the museum uses it

`internal/museum/analyzer.go` sends each wav segment it downloads to `POST /transcribe` before queueing it to play,
and gets back

```json
{
  "notes": [{ "pitch": 60, "start": 0.12, "duration": 0.4, "velocity": 96 }],
  "duration": 2,
  "analysisSeconds": 0.31
}
```

with `pitch` as a MIDI note (21 to 108, the 88 keys), and `start` and `duration` in seconds into the segment. The
projector lights the keys from the same position in the audio the speakers are at; it doesn't play a second copy.
`GET /health` says whether it's ready and busy.

What to expect:

- recognition is an estimate: chords, the sustain pedal, background noise and quiet notes can come out wrong, and
  notes held across two segments are only approximate
- it adds a delay: if it's slower than the stream, the audio pauses between segments while it catches up
- a segment it can't analyse, or every segment while it isn't running, still plays, with the keyboard unlit. The
  museum tries it again 10 s after a failure
- the wav is written to a temporary folder for the model and deleted right after
