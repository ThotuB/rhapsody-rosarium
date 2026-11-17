# Piano simulator

Run ten pianos, with **piano-2 / Piața Libertății** looping your WAV recording:

```powershell
cd C:\Users\Robert\Documents\Unihack\rhapsody-rosarium\node-sim
uv run --python 3.13 simulate.py --nodes 10 --wav "C:\Users\Robert\Documents\Unihack\audiomass-output.wav" --wav-piano piano-2
```

Stop the old simulator first with Ctrl+C. Do not run two simulators uploading
under the same piano ID. Other pianos continue generating music as before.

For just the recorded piano:

```powershell
uv run --python 3.13 simulate.py --id piano-2 --wav "C:\Users\Robert\Documents\Unihack\audiomass-output.wav"
```

FFmpeg must be on this terminal's PATH, just as for the server. The WAV is decoded
once to mono 48 kHz PCM. Consecutive segments preserve the samples and loop at the
end of the recording, even if a segment crosses the end of the file. The seam
between the recording's end and beginning may be audible. Normal simulated
recording/upload timing and sequence numbers are retained. Avoid `--fast` for
listening tests.

Keep the server and the museum's transcriber (`client-museum/transcriber`) running, then choose
**Piața Libertății** in the museum window. The recording goes through the same
server, recognition, speaker playback, and projector-keyboard pipeline as live
piano-node audio. You may join in the middle of the recording because the museum
plays recent segments rather than restarting the recording when you tune in.

You can use another active ID with `--wav-piano`. Paths supplied through a JSON
scenario's `wav` field resolve relative to the current working directory. The
optional `wav_piano` field defaults to `piano-2`.

Unit checks for looping, segment boundaries and configuration (no server needed):

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests -p test_wav_source.py
```
