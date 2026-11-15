import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import wave

import numpy as np

import config
from music import RATE
from wav_source import WavVoice


def decode(data):
    with wave.open(io.BytesIO(data)) as wav:
        assert (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) == (1, 2, RATE)
        return np.frombuffer(wav.readframes(wav.getnframes()), dtype="<i2").tolist()


class WavSourceTests(unittest.TestCase):
    def test_segments_continue_and_wrap_without_dropped_or_padded_samples(self):
        voice = WavVoice(np.array([100, -100, 1200, -200], dtype="<i2").tobytes())
        self.assertEqual(decode(voice.segment(3 / RATE)), [100, -100, 1200])
        self.assertEqual(decode(voice.segment(3 / RATE)), [-200, 100, -100])
        self.assertEqual(decode(voice.segment(6 / RATE)), [1200, -200, 100, -100, 1200, -200])

    def test_empty_recording_is_rejected(self):
        with self.assertRaises(ValueError):
            WavVoice(b"")

    def test_selected_piano_and_path_are_resolved(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            path = Path(directory) / "recording.wav"
            path.write_bytes(b"wav")
            with patch("sys.argv", ["simulate.py", "--nodes", "3", "--wav", str(path)]):
                settings, pianos = config.load()
            self.assertEqual(settings.wav, path.resolve())
            self.assertEqual(settings.wav_piano, "piano-2")
            self.assertEqual(len(pianos), 3)

    def test_muted_wav_target_is_rejected(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            path = Path(directory) / "recording.wav"
            path.write_bytes(b"wav")
            with patch("sys.argv", ["simulate.py", "--wav", str(path), "--mute", "piano-2"]):
                with patch("sys.stderr", io.StringIO()), self.assertRaises(SystemExit):
                    config.load()


if __name__ == "__main__":
    unittest.main()
