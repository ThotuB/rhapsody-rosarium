import io
import itertools
import wave

import numpy as np

from music import LOUDNESS_LEVELS, Voice


def samples(wav: bytes) -> np.ndarray:
    with wave.open(io.BytesIO(wav)) as w:
        return np.frombuffer(w.readframes(w.getnframes()), "<i2") / 32767


def melody(node: str, minutes: int = 5) -> list[int]:
    """the melody's positions in the scale over a few minutes"""
    voice = Voice.for_node(node)
    notes = voice.notes()
    return [notes.index(m) for _ in range(minutes * 6) for _, m, _ in voice.compose(10)]


def test_same_piano_same_music():
    assert Voice.for_node("piano-3").segment(5) == Voice.for_node("piano-3").segment(5)
    assert Voice.for_node("piano-3").segment(5) != Voice.for_node("piano-4").segment(5)


def test_segments_join_without_a_seam():
    voice = Voice.for_node("piano-3")
    segments = [samples(voice.segment(4)) for _ in range(3)]
    joined = np.concatenate(segments)
    steps = np.abs(np.diff(joined))
    for k in (1, 2):
        join = k * len(segments[0])
        assert steps[join - 1] <= np.percentile(steps, 99.9)
    assert np.mean(np.abs(joined) > 0.95) < 0.001  # the soft limit rarely kicks in


def test_composing_in_segments_matches_one_long_stretch():
    split_voice, whole_voice = Voice.for_node("piano-5"), Voice.for_node("piano-5")
    split = [
        (k * 10 + t, m, l) for k in range(3) for t, m, l in split_voice.compose(10)
    ]
    whole = whole_voice.compose(30)
    assert len(split) == len(whole)
    for (t1, m1, l1), (t2, m2, l2) in zip(split, whole):
        assert m1 == m2
        assert abs(t1 - t2) < 1e-6 and abs(l1 - l2) < 1e-6


def test_melody_moves_in_small_steps_and_covers_the_range():
    for node in ("piano-1", "piano-2", "piano-3"):
        degrees = melody(node)
        steps = np.abs(np.diff(degrees))
        assert steps.max() <= 2
        assert 0.05 < np.mean(steps == 0) < 0.3  # repeats happen, but don't dominate
        assert max(len(list(run)) for _, run in itertools.groupby(degrees)) <= 10
        assert set(degrees) == set(range(14))


def test_melody_forms_lines():
    for node in ("piano-1", "piano-2", "piano-3"):
        moves = [np.sign(s) for s in np.diff(melody(node)) if s != 0]
        lengths = [len(list(run)) for _, run in itertools.groupby(moves)]
        assert np.mean(lengths) > 2.5  # notes moving the same way before turning


def test_loudness_holds_on_levels():
    voice = Voice.for_node("piano-2")
    loudness = np.array([l for _ in range(30) for _, _, l in voice.compose(10)])
    assert np.mean(np.diff(loudness) == 0) > 0.5
    levels = 0.3 + 0.7 * np.linspace(0, 1, LOUDNESS_LEVELS)
    held = loudness[np.isclose(loudness[:, None], levels).any(axis=1)]
    assert len(np.unique(held.round(6))) >= 3
