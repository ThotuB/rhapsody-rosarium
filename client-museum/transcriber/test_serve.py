import io
import unittest
from typing import Any, cast

import numpy as np
import soundfile

from serve import MAX_SECONDS, Rejected, normalize_notes, transcribe

# what isn't a recording it can take is turned down before Basic Pitch would see it
NO_MODEL = cast(Any, None)


def wav(seconds):
    data = io.BytesIO()
    soundfile.write(data, np.zeros(int(seconds * 8000)), 8000, format="WAV")
    return data.getvalue()


class NoteConversionTests(unittest.TestCase):
    def test_chord_amplitudes_and_duration_clipping(self):
        notes = normalize_notes([(0.5, 2.5, 64, 0.5, None), (-0.1, 1, 60, 1, None)], 2)
        self.assertEqual(
            notes,
            [
                {"pitch": 60, "start": 0, "duration": 1, "velocity": 127},
                {"pitch": 64, "start": 0.5, "duration": 1.5, "velocity": 64},
            ],
        )

    def test_empty_and_out_of_range_notes(self):
        self.assertEqual(normalize_notes([], 1), [])
        self.assertEqual(
            normalize_notes([(0, 1, 10, 1, None), (2, 3, 60, 1, None)], 1), []
        )


class RejectionTests(unittest.TestCase):
    def test_what_isnt_a_wav(self):
        for upload in (b"", b"ID3\x04 an mp3", b"RIFF\0\0\0\0WAVE but no audio"):
            with self.assertRaises(Rejected) as caught:
                transcribe(NO_MODEL, upload)
            self.assertEqual(caught.exception.status, 400)

    def test_too_long_or_empty(self):
        for seconds in (0, MAX_SECONDS + 1):
            with self.assertRaises(Rejected) as caught:
                transcribe(NO_MODEL, wav(seconds))
            self.assertEqual(caught.exception.status, 400)


if __name__ == "__main__":
    unittest.main()
