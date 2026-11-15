import unittest

from serve import normalize_notes


class NoteConversionTests(unittest.TestCase):
    def test_chord_amplitudes_and_duration_clipping(self):
        notes = normalize_notes([(0.5, 2.5, 64, 0.5, None), (-0.1, 1, 60, 1, None)], 2)
        self.assertEqual(notes, [
            {"pitch": 60, "start": 0, "duration": 1, "velocity": 127},
            {"pitch": 64, "start": 0.5, "duration": 1.5, "velocity": 64},
        ])

    def test_empty_and_out_of_range_notes(self):
        self.assertEqual(normalize_notes([], 1), [])
        self.assertEqual(normalize_notes([(0, 1, 10, 1, None), (2, 3, 60, 1, None)], 1), [])


if __name__ == "__main__":
    unittest.main()
