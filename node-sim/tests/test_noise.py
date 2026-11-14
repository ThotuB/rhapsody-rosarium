import numpy as np

from noise import Noise

TIMES = np.arange(0, 2000, 0.25)


def flip_backs(slots, within=20):
    """changes undone again within a few readings, like A, B, A"""
    changes = [i for i in range(1, len(slots)) if slots[i] != slots[i - 1]]
    return sum(
        1
        for a, b in zip(changes, changes[1:])
        if b - a <= within and slots[b] == slots[a - 1]
    )


def test_values_stay_in_range_and_spread_evenly():
    values = np.array([Noise("a", speed=0.5).at(t) for t in TIMES])
    assert values.min() >= 0 and values.max() <= 1
    # stretched: the bottom and top tenths come up about as often as the middle ones
    counts, _ = np.histogram(values, bins=10, range=(0, 1))
    assert counts.min() > 0.5 * counts.mean()


def test_curve_is_smooth():
    noise = Noise("a", speed=1.0)
    steps = [abs(noise.at(t + 0.01) - noise.at(t)) for t in np.arange(0, 300, 0.37)]
    assert max(steps) < 0.1


def test_same_seed_same_curve():
    a, b, other = Noise("x", 0.5), Noise("x", 0.5), Noise("y", 0.5)
    assert all(a.at(t) == b.at(t) for t in TIMES[:400])
    assert any(a.at(t) != other.at(t) for t in TIMES[:400])


def test_index_holds_a_slot_near_its_edges():
    # read densely, so the curve can hover on edges between readings
    noise = Noise("a", speed=2.0)
    times = np.arange(0, 100, 0.005)
    free = [noise.index(t, 7) for t in times]
    sticky, held = [], None
    for t in times:
        held = noise.index(t, 7, held=held, margin=0.25)
        sticky.append(held)
    assert flip_backs(sticky) < 0.7 * flip_backs(free)
    # holding never strays more than one slot from where the curve is
    assert max(abs(a - b) for a, b in zip(free, sticky)) <= 1


def test_pick_follows_index():
    noise = Noise("a", speed=1.0)
    options = ("low", "mid", "high")
    assert all(
        noise.pick(t, options) == options[noise.index(t, 3)] for t in TIMES[:400]
    )


def test_terraced_rests_on_levels():
    noise = Noise("a", speed=0.5)
    values = np.array([noise.terraced(t, 4) for t in TIMES])
    levels = np.linspace(0, 1, 4)
    on_a_level = np.isclose(values[:, None], levels, atol=1e-9).any(axis=1)
    assert on_a_level.mean() > 0.6
    assert values.min() >= 0 and values.max() <= 1
