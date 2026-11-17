import math
from time import sleep

from gpiozero import DigitalOutputDevice, PWMOutputDevice

# RGB channels (active LOW), pins as wired (see test/channels.py)
red = DigitalOutputDevice(22, initial_value=True)
green = DigitalOutputDevice(17, initial_value=True)
blue = DigitalOutputDevice(27, initial_value=True)

# PWM brightness control
ena = PWMOutputDevice(5, frequency=500, initial_value=0)
enb = PWMOutputDevice(6, frequency=500, initial_value=0)


def purple(brightness):
    """Purple brightness from 0.0 (OFF) to 1.0 (FULL)."""
    brightness = max(0.0, min(1.0, brightness))

    ena.value = brightness
    enb.value = brightness


def breathe(duration=4, steps=100):
    """Smoothly fade purple OFF -> ON -> OFF."""
    while True:
        for i in range(steps + 1):
            progress = i / steps

            intensity = (1 - math.cos(2 * math.pi * progress)) / 2

            purple(intensity)
            sleep(duration / steps)


try:
    ena.value = 1
    enb.value = 1
    # Fixed color: RED + BLUE = PURPLE
    red.off()  # IN1 LOW, red channel ON
    green.on()  # IN2 HIGH, green channel OFF
    blue.off()  # IN3 LOW, blue channel ON

    print("Purple breathing effect started")
    breathe(duration=4)

except KeyboardInterrupt:
    print("\nStopping LED")

finally:
    purple(0)
    ena.close()
    enb.close()
    red.close()
    green.close()
    blue.close()
