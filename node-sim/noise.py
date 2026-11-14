"""smooth random curves over time, so the fake pianos' music moves instead of jumping.

a Noise is 1d gradient (perlin) noise: random slopes at whole-number points, blended
smoothly in between, with a second finer layer added for more natural motion. its
values are stretched so every part of [0, 1] comes up equally often over time, so the
top and bottom of a range get used as much as the middle. seeded, so a curve is the
same on every run.
"""

import math
import random

import numpy as np

LATTICE = 4096  # random slopes; a curve repeats after this many ups and downs
LAYERS = ((1.0, 1.0), (2.0, 0.5))  # (frequency, weight) of the layers added together


class Noise:
    def __init__(self, seed: str, speed: float):
        """speed: roughly how many ups and downs per second"""
        rng = random.Random(seed)
        self.speed = speed
        self.slopes = np.array([rng.uniform(-1, 1) for _ in range(LATTICE)])
        self.offset = rng.uniform(0, LATTICE)
        # how the raw values are spread, to stretch them evenly over [0, 1]
        self.spread = np.sort(self._raw(np.arange(8192) * 0.37))
        self.even = np.linspace(0, 1, len(self.spread))

    def at(self, t: float) -> float:
        """the curve at time t (seconds), in [0, 1]"""
        raw = self._raw(np.asarray(t * self.speed, dtype=float))
        return float(np.interp(raw, self.spread, self.even))

    def index(
        self, t: float, n: int, held: int | None = None, margin: float = 0.0
    ) -> int:
        """one of n slots, with neighbouring times landing on neighbouring slots.

        pass the slot picked last time as `held` to keep it until the curve has moved
        more than `margin` (a fraction of a slot) past its edge, so a curve hovering
        on an edge doesn't flip back and forth between two slots."""
        x = self.at(t) * n
        if held is not None and held - margin <= x < held + 1 + margin:
            return held
        return min(int(x), n - 1)

    def pick[T](self, t: float, options: list[T] | tuple[T, ...]) -> T:
        """one of the options, with neighbouring times picking neighbouring options"""
        return options[self.index(t, len(options))]

    def terraced(self, t: float, levels: int, slide: float = 0.3) -> float:
        """like at(), but resting on `levels` evenly spaced values in [0, 1] and moving
        quickly between them: it holds a level for a while, moves, then holds again.
        `slide` is how much of the way between two levels is spent moving."""
        x = self.at(t) * (levels - 1)
        k = min(math.floor(x), levels - 2)
        s = (x - k - (0.5 - slide / 2)) / slide
        s = min(max(s, 0.0), 1.0)
        return (k + s * s * (3 - 2 * s)) / (levels - 1)

    def _raw(self, x: np.ndarray) -> np.ndarray:
        return sum(
            weight * self._layer((x + self.offset) * freq) for freq, weight in LAYERS
        )

    def _layer(self, x: np.ndarray) -> np.ndarray:
        cell = np.floor(x)
        f = x - cell
        i = cell.astype(int) % LATTICE
        a = self.slopes[i] * f
        b = self.slopes[(i + 1) % LATTICE] * (f - 1)
        fade = f * f * f * (f * (f * 6 - 15) + 10)
        return a + fade * (b - a)
