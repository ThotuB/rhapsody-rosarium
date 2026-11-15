# Piano recognition and projector

## Live projector (default)

The projector opens with the video at `media/intro.mp4`, relative to the museum
working directory (`client-museum`, the normal launch). It's in the repository through
Git LFS (`git lfs install` once, then `git lfs pull` if a clone has only the pointer
file), and served directly from disk rather than embedded into each executable.
Another video with:

```powershell
go run . -lights off -video "C:\path\to\another.mp4"
```

On Linux the projector's web view needs an H.264 decoder for GStreamer
(`sudo dnf install gstreamer1-plugin-openh264` on Fedora).

The video autoplays muted; **Enable video sound** enables its audio. When it ends,
it fades into the piano map and the 88-key keyboard appears along the bottom.
**Skip intro** and **Replay intro** are available in the top-right controls, and
**Restart intro** in the museum window.
If the file cannot be loaded, **Continue to map** still opens the live view.
The map needs internet access for its tiles.

Start the AI helper using the setup below, keep the Go server and piano nodes
(or simulator) running, and select a piano in the museum window. The museum
analyzes received WAV segments before queuing them for speaker playback. The
projector highlights the keys from the same sample position the speakers use;
it does not play a second copy of the audio. Video playback does not stop the
selected piano's audio: mute the video or stop the piano from the museum window
if you want to hear only one source during the introduction.

Recognition adds processing delay. If it is slower than the incoming stream,
audio may pause between segments while analysis catches up. If the helper is
unavailable or a segment fails analysis, that audio still plays and the keyboard
stays unlit for that segment, with a status message. No recognition runs in the
sound-card callback. Recognition of notes spanning segment boundaries is still
approximate.

## On Linux

Set up once (Python 3.10 through uv; on Linux Basic Pitch runs its TFLite model, the
"Tensorflow is not installed" warning is harmless):

```sh
cd client-museum/transcription
uv venv --python 3.10 .venv
uv pip install --python .venv/bin/python -r requirements.txt
```

Start it, and keep it running next to the museum app:

```sh
.venv/bin/python serve.py
```

It's ready after a few seconds (the very first start compiles parts of the libraries and
can take much longer). The first segment takes ~3 s while the model warms up, then each
2 s segment takes ~0.3 s.

## 1. Set up the AI helper once (PowerShell)

Install **64-bit Python 3.10** from https://www.python.org/downloads/release/python-31011/
if `py -3.10 --version` does not work. This is intentionally separate from the
newer Python versions used by the other nodes: Basic Pitch 0.4 supports Python up
to 3.11, and uses ONNX on Windows with Python 3.10.

```powershell
cd C:\Users\Robert\Documents\Unihack\rhapsody-rosarium\client-museum\transcription
py -3.10 -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
```

No environment activation is required, so PowerShell's script execution policy
does not interfere. The AI model comes with Basic Pitch; no API key is needed.

## 2. Start the helper

```powershell
cd C:\Users\Robert\Documents\Unihack\rhapsody-rosarium\client-museum\transcription
.\.venv\Scripts\python.exe serve.py
```

Wait for **Ready at http://127.0.0.1:8765**. Keep this terminal open. You can also
visit http://127.0.0.1:8765/health to check it. If the port is already in use,
close the previous helper before starting another.

## 3. Rebuild the frontend, then launch the museum

In another PowerShell terminal, run the following commands yourself:

```powershell
fnm env --use-on-cd --shell powershell | Out-String | Invoke-Expression
fnm use 26
cd C:\Users\Robert\Documents\Unihack\rhapsody-rosarium\client-museum\frontend
npm.cmd ci
npm.cmd run build
cd ..
$env:Path = "D:\Program Files\MSYSK\ucrt64\bin;$env:Path"
$env:CC = "gcc.exe"
$env:CXX = "g++.exe"
$env:CGO_ENABLED = "1"
go run . -lights off
```

Close the old museum app before relaunching: Go embeds the frontend at build
time. Keep the Go server, AI helper and simulator (or real piano nodes) running.

## 4. Stream a WAV as one piano

The standalone WAV test screen has been removed. Instead, give the simulator a
recording to loop as piano-2 while the other pianos keep generating music:

```powershell
cd C:\Users\Robert\Documents\Unihack\rhapsody-rosarium\node-sim
uv run --python 3.13 simulate.py --nodes 10 --wav "C:\Users\Robert\Documents\Unihack\audiomass-output.wav" --wav-piano piano-2
```

Stop any previous simulator first. Select **Piața Libertății** in the museum
window to hear that recording and see the projector keyboard follow it. See
[the simulator guide](../../node-sim/README.md) for details.

Go sends incoming segments only to the AI helper on your own computer. Temporary
uploaded files are deleted after analysis. Recognition is an estimate: chords,
sustain, background noise, and quiet notes can be wrong.

Model documentation: https://github.com/spotify/basic-pitch
