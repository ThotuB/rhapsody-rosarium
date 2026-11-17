"""RGB LED that pulses faster the more people listen to this piano, in the colour the museum sets.

app.py's heartbeat passes on the server's reply: the listener count (0 listeners
is off, 1 pulses slowly, and every extra listener pulses faster) and the colour,
"#rrggbb". The light runs in its own thread.
"""

import math
import threading
import time

from gpiozero import DigitalOutputDevice, PWMOutputDevice

# Pulse configuration
PULSES_PER_LISTENER = (
    0.25  # Pulses per second added by each listener: 1 -> every 4 s, 2 -> every 2 s
)
MAX_PULSES = 2.5  # Pulses per second at most, from 10 listeners on
FADE_SPEED = (
    0.6  # How fast the light fades in / out when listeners come and go, per second
)
UPDATE_INTERVAL = 0.02

# Colour configuration
DEFAULT_COLOUR = (
    "#ff00ff"  # Purple, until the museum sets another (the server's default too)
)
COLOUR_FADE = (
    2.0  # How fast a new colour fades in, per second: half a second from one to another
)


def pulses_for(listeners):
    """Pulses per second for a listener count, 0 for nobody."""
    return min(max(0, listeners) * PULSES_PER_LISTENER, MAX_PULSES)


def parse_colour(text):
    """(red, green, blue) from 0.0 to 1.0 for "#rrggbb", None if it isn't one."""
    if not isinstance(text, str) or len(text) != 7 or text[0] != "#":
        return None
    try:
        value = int(text[1:], 16)
    except ValueError:
        return None
    return tuple(((value >> shift) & 0xFF) / 255 for shift in (16, 8, 0))


class Light:
    """RGB strip on the L298N: a PWM per colour on its inputs, with ENA / ENB held on."""

    def __init__(self):
        # Both halves of the L298N on; colour and brightness are all in the colour PWMs
        self.ena = DigitalOutputDevice(5, initial_value=True)
        self.enb = DigitalOutputDevice(6, initial_value=True)

        # RGB channels, active LOW: a value of 1 holds the pin low, that colour fully on.
        # Pins as wired, checked with test/channels.py: GPIO 22 -> IN3 red,
        # 17 -> IN1 green, 27 -> IN2 blue
        self.channels = [
            PWMOutputDevice(pin, active_high=False, frequency=500, initial_value=0)
            for pin in (22, 17, 27)  # Red, green, blue
        ]

        self.lock = threading.Lock()
        self.pulses = 0.0  # Pulses per second, 0 is off
        self.colour = parse_colour(DEFAULT_COLOUR)  # The colour to show, (r, g, b)
        self.thread = None

    def start(self, stop):
        """Pulse in a thread of its own until stop is set."""
        self.thread = threading.Thread(target=self.run, args=(stop,), daemon=True)
        self.thread.start()

    def show(self, listeners, colour=None):
        """Pulse at the speed for this many listeners, in colour ("#rrggbb").

        A missing or invalid colour keeps the current one.
        """
        rgb = parse_colour(colour)
        with self.lock:
            self.pulses = pulses_for(listeners)
            if rgb is not None:
                self.colour = rgb

    def run(self, stop):
        """Pulse until stop is set."""
        phase = 0.0  # Position in the pulse, 0 to 1: speed changes carry on from here, no jumps
        level = 0.0  # Fades to 1 while anyone listens, to 0 once nobody does
        rate = 0.0
        with self.lock:
            shown = list(
                self.colour
            )  # The colour shown now, fading towards self.colour
        last_time = time.monotonic()

        while not stop.is_set():
            now = time.monotonic()
            dt = now - last_time
            last_time = now

            with self.lock:
                # Nobody listening: finish fading out at the last speed
                rate = self.pulses or rate
                on = self.pulses > 0
                target = self.colour

            phase = (phase + rate * dt) % 1.0

            if on:
                level = min(1.0, level + FADE_SPEED * dt)
            else:
                level = max(0.0, level - FADE_SPEED * dt)

            # A new colour fades in instead of jumping
            step = COLOUR_FADE * dt
            shown = [c + max(-step, min(step, t - c)) for c, t in zip(shown, target)]

            # Smoothly OFF -> ON -> OFF once per pulse
            brightness = level * (1 - math.cos(2 * math.pi * phase)) / 2

            for channel, c in zip(self.channels, shown):
                channel.value = brightness * c

            stop.wait(UPDATE_INTERVAL)

    def close(self):
        """Switch off, once the pulsing thread saw stop."""
        if self.thread is not None:
            self.thread.join(timeout=1)

        for channel in self.channels:
            channel.value = 0

        for device in (self.ena, self.enb, *self.channels):
            device.close()


def open_light():
    """The light, or None without GPIO pins (e.g. on a laptop), so recording still runs."""
    try:
        return Light()

    # gpiozero raises several kinds of error when it can't reach the pins
    except Exception as exc:
        print(f"LED unavailable, running without it: {exc}", flush=True)
        return None
