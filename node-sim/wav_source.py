"""Turn a recording into consecutive, looping 48 kHz PCM WAV segments."""

import io
import subprocess
import wave
from pathlib import Path

import numpy as np

from music import RATE


class WavVoice:
    def __init__(self, pcm: bytes):
        if not pcm or len(pcm) % 2:
            raise ValueError("The WAV recording contains no complete audio samples.")
        self.samples = np.frombuffer(pcm, dtype="<i2")
        self.position = 0

    @classmethod
    def from_file(cls, path: Path):
        # Match the server's mono 48 kHz format once before starting any nodes.
        # FFmpeg handles stereo/float/24-bit WAVs as well as ordinary PCM files.
        try:
            result = subprocess.run(
                [
                    "ffmpeg",
                    "-nostdin",
                    "-v",
                    "error",
                    "-i",
                    str(path),
                    "-vn",
                    "-ac",
                    "1",
                    "-ar",
                    str(RATE),
                    "-c:a",
                    "pcm_s16le",
                    "-f",
                    "s16le",
                    "pipe:1",
                ],
                capture_output=True,
                check=True,
                timeout=120,
            )
        except FileNotFoundError as error:
            raise ValueError(
                "FFmpeg is required for --wav. Add ffmpeg.exe to this terminal's PATH."
            ) from error
        except subprocess.CalledProcessError as error:
            raise ValueError(
                f"Cannot decode WAV: {error.stderr.decode(errors='replace')[:500]}"
            ) from error
        except subprocess.TimeoutExpired as error:
            raise ValueError(
                "WAV decoding timed out. Try a shorter recording."
            ) from error
        return cls(result.stdout)

    def segment(self, seconds: float) -> bytes:
        count = round(RATE * seconds)
        if count < 1:
            raise ValueError("Segment duration is shorter than one sample.")
        indexes = (np.arange(count) + self.position) % len(self.samples)
        pcm = self.samples[indexes].tobytes()
        self.position = (self.position + count) % len(self.samples)
        output = io.BytesIO()
        with wave.open(output, "wb") as wav:
            wav.setnchannels(1)
            wav.setsampwidth(2)
            wav.setframerate(RATE)
            wav.writeframes(pcm)
        return output.getvalue()
