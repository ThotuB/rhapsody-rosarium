"""synthesised piano-ish audio for the fake nodes.

each piano gets its own key, scale and tempo, chosen at random once. after that
nothing is random note by note: smooth noise curves (see noise.py) decide the pitch,
loudness and pace over time, so neighbouring notes belong together and the music
forms lines, louder and softer passages, and busier and calmer stretches."""

import io
import random
import wave
from dataclasses import dataclass, field
from typing import Self

import numpy as np

from noise import Noise

RATE = 48000  # what the server stores and the web client plays at, so nothing gets resampled
MAJOR = [0, 2, 4, 5, 7, 9, 11]
MINOR = [0, 2, 3, 5, 7, 8, 10]
STEPS = (-2, -1, -1, 0, 1, 1, 2)  # how the melody moves per note, in scale steps
GAPS = (2, 1, 0.5)  # beats between notes, from calm to busy
LOUDNESS_LEVELS = 4  # the dynamics rest on this many levels, soft to loud
STICKY = 0.25  # how far (in slots) a curve must go past a choice's edge to change it


def note(freq, seconds):
    t = np.arange(int(RATE * seconds)) / RATE
    tone = sum(
        a * np.sin(2 * np.pi * freq * k * t)
        for k, a in ((1, 1.0), (2, 0.5), (3, 0.25), (4, 0.12))
    )
    return tone * np.exp(-3.0 * t) * np.minimum(t / 0.005, 1.0)


def to_wav(samples):
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes((samples * 32767).astype("<i2").tobytes())
    return buf.getvalue()


@dataclass
class Voice:
    """a fake piano's musical character, and where its music left off.

    consecutive segments continue each other: the curves are read on a running
    clock, notes still ringing at the end of one carry into the next, and the
    rhythm doesn't restart, so the segments join without a seam when played back
    to back."""

    tonic: int  # midi note the scale is built on, i.e. the key
    scale: list[int]  # semitones above the tonic
    beat: float  # seconds between note starts, before the pace varies it
    melody: Noise  # which way the melody moves, read per note
    dynamics: Noise  # how loud: the slowest curve, read terraced
    density: Noise  # how busy: picks the gap to the next note
    clock: float = 0.0  # seconds of music made so far
    played: int = 0  # notes made so far, where the melody curve is read
    degree: int = 7  # current position in notes(), starting in the middle
    flip: int = 1  # -1 while the melody's motion is mirrored after reaching an edge
    motion: int | None = None  # current position in STEPS, held against wobbles
    pace: int | None = None  # current position in GAPS, held the same way
    tail: np.ndarray = field(default_factory=lambda: np.zeros(0))  # still ringing
    next_note: float = 0.0  # seconds into the next segment where the next note starts

    @classmethod
    def for_node(cls, node_id: str) -> Self:
        """seeded by the id, so a piano sounds the same on every run"""
        rng = random.Random(node_id)
        tonic = rng.randint(45, 62)
        scale = rng.choice((MAJOR, MINOR))
        beat = rng.uniform(0.3, 0.8)
        return cls(
            tonic=tonic,
            scale=scale,
            beat=beat,
            # the melody changes direction every 6-10 notes or so
            melody=Noise(f"{node_id}/melody", speed=1 / rng.uniform(6, 10)),
            # loudness changes every 20-40 s, pace every 10-20 s
            dynamics=Noise(f"{node_id}/dynamics", speed=1 / rng.uniform(20, 40)),
            density=Noise(f"{node_id}/density", speed=1 / rng.uniform(10, 20)),
        )

    def notes(self) -> list[int]:
        """the midi notes the melody moves over: two octaves of the scale, low to high"""
        return [self.tonic + 12 * octave + s for octave in (0, 1) for s in self.scale]

    def compose(self, seconds: float) -> list[tuple[float, int, float]]:
        """the notes starting in the next `seconds`, as (start, midi note, loudness)"""
        notes = self.notes()
        events = []
        pos = self.next_note
        while pos < seconds:
            t = self.clock + pos
            self.move(len(notes))
            loudness = 0.3 + 0.7 * self.dynamics.terraced(t, LOUDNESS_LEVELS)
            events.append((pos, notes[self.degree], loudness))
            self.pace = self.density.index(t, len(GAPS), held=self.pace, margin=STICKY)
            pos += self.beat * GAPS[self.pace]
        self.next_note = pos - seconds
        self.clock += seconds
        return events

    def move(self, n: int):
        """step to the next melody note. the curve decides which way and how far to
        move rather than which note to play, so the melody rises and falls in lines
        instead of jumping between far apart notes"""
        self.motion = self.melody.index(
            self.played, len(STEPS), held=self.motion, margin=STICKY
        )
        step = STEPS[self.motion] * self.flip
        if not 0 <= self.degree + step < n:
            # mirror the motion at the top or bottom until the next edge, so the line
            # turns around there instead of bouncing back and forth
            self.flip = -self.flip
            step = -step
        self.degree += step
        self.played += 1

    def segment(self, seconds: float) -> bytes:
        """the next `seconds` of this piano's music as a wav"""
        n = int(RATE * seconds)
        out = np.zeros(n + RATE * 2)  # room for the last notes to ring out
        out[: len(self.tail)] += self.tail
        for pos, midi, loudness in self.compose(seconds):
            tone = note(440 * 2 ** ((midi - 69) / 12), 2.0)
            i = int(pos * RATE)
            out[i : i + len(tone)] += tone * loudness
        self.tail = out[n:].copy()
        # a fixed gain with a soft limit instead of normalising each segment,
        # which would make the volume jump at every join
        return to_wav(np.tanh(out[:n] * 0.4))
