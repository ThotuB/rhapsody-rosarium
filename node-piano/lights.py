"""Purple LED whose brightness follows how many people listen to this piano.

record.py's heartbeat passes on the listener count from the server's reply,
the light fades towards the matching brightness in its own thread.
"""

import threading
import time

from gpiozero import DigitalOutputDevice, PWMOutputDevice


# Brightness configuration
MIN_BRIGHTNESS = 0
MAX_LISTENERS = 3  # Full brightness from this many listeners
FADE_SPEED = 0.6  # Brightness units per second
UPDATE_INTERVAL = 0.05


def brightness_for(listeners):
    """Map listener count to brightness (0.0 to 1.0)."""
    listeners = max(0, min(listeners, MAX_LISTENERS))

    return MIN_BRIGHTNESS + (
        (1.0 - MIN_BRIGHTNESS)
        * listeners / MAX_LISTENERS
    )


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
        self.target = MIN_BRIGHTNESS
        self.thread = None

    def start(self, stop):
        """Fade in a thread of its own until stop is set."""
        self.thread = threading.Thread(
            target=self.run,
            args=(stop,),
            daemon=True
        )
        self.thread.start()

    def show(self, listeners):
        """Fade to the brightness for this many listeners."""
        with self.lock:
            self.target = brightness_for(listeners)

    def run(self, stop):
        """Gradually approach the target brightness until stop is set."""
        current = 0.0
        last_time = time.monotonic()

        while not stop.is_set():
            now = time.monotonic()
            max_change = FADE_SPEED * (now - last_time)
            last_time = now

            with self.lock:
                difference = self.target - current

            if abs(difference) <= max_change:
                current += difference
            elif difference > 0:
                current += max_change
            else:
                current -= max_change

            current = max(0.0, min(1.0, current))
            self.ena.value = current
            self.enb.value = current

            stop.wait(UPDATE_INTERVAL)

    def close(self):
        """Switch off, once the fading thread saw stop."""
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
