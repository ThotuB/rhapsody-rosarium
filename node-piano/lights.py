"""Purple LED that pulses faster the more people listen to this piano.

app.py's heartbeat passes on the listener count from the server's reply:
0 listeners is off, 1 pulses slowly, and every extra listener pulses faster.
The light runs in its own thread.
"""

import math
import threading
import time

from gpiozero import DigitalOutputDevice, PWMOutputDevice


# Pulse configuration
PULSES_PER_LISTENER = 0.25  # Pulses per second added by each listener: 1 -> every 4 s, 2 -> every 2 s
MAX_PULSES = 2.5  # Pulses per second at most, from 10 listeners on
FADE_SPEED = 0.6  # How fast the light fades in / out when listeners come and go, per second
UPDATE_INTERVAL = 0.02


def pulses_for(listeners):
    """Pulses per second for a listener count, 0 for nobody."""
    return min(max(0, listeners) * PULSES_PER_LISTENER, MAX_PULSES)


class Light:
    """Purple (red + blue) on the L298N, brightness by PWM on ENA / ENB."""

    def __init__(self):
        self.ena = PWMOutputDevice(5, frequency=500, initial_value=0)
        self.enb = PWMOutputDevice(6, frequency=500, initial_value=0)

        # RGB channels (active LOW): red + blue = purple
        self.red = DigitalOutputDevice(17, initial_value=False)  # ON
        self.green = DigitalOutputDevice(27, initial_value=True)  # OFF
        self.blue = DigitalOutputDevice(22, initial_value=False)  # ON

        self.lock = threading.Lock()
        self.pulses = 0.0  # Pulses per second, 0 is off
        self.thread = None

    def start(self, stop):
        """Pulse in a thread of its own until stop is set."""
        self.thread = threading.Thread(
            target=self.run,
            args=(stop,),
            daemon=True
        )
        self.thread.start()

    def show(self, listeners):
        """Pulse at the speed for this many listeners."""
        with self.lock:
            self.pulses = pulses_for(listeners)

    def run(self, stop):
        """Pulse until stop is set."""
        phase = 0.0  # Position in the pulse, 0 to 1: speed changes carry on from here, no jumps
        level = 0.0  # Fades to 1 while anyone listens, to 0 once nobody does
        rate = 0.0
        last_time = time.monotonic()

        while not stop.is_set():
            now = time.monotonic()
            dt = now - last_time
            last_time = now

            with self.lock:
                # Nobody listening: finish fading out at the last speed
                rate = self.pulses or rate
                on = self.pulses > 0

            phase = (phase + rate * dt) % 1.0

            if on:
                level = min(1.0, level + FADE_SPEED * dt)
            else:
                level = max(0.0, level - FADE_SPEED * dt)

            # Smoothly OFF -> ON -> OFF once per pulse
            brightness = level * (1 - math.cos(2 * math.pi * phase)) / 2

            self.ena.value = brightness
            self.enb.value = brightness

            stop.wait(UPDATE_INTERVAL)

    def close(self):
        """Switch off, once the pulsing thread saw stop."""
        if self.thread is not None:
            self.thread.join(timeout=1)

        self.ena.value = 0
        self.enb.value = 0

        for device in (self.ena, self.enb, self.red, self.green, self.blue):
            device.close()


def open_light():
    """The light, or None without GPIO pins (e.g. on a laptop), so recording still runs."""
    try:
        return Light()

    # gpiozero raises several kinds of error when it can't reach the pins
    except Exception as exc:
        print(f"LED unavailable, running without it: {exc}", flush=True)
        return None
