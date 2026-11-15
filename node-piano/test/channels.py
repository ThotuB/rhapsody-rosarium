"""Light the LED's channels one at a time, to see which wire is which.

Uses the same pins as lights.py; stop app.py first, it holds them.

    uv run test/channels.py
"""

from time import sleep

from gpiozero import DigitalOutputDevice, PWMOutputDevice

# L298N wiring as in lights.py: IN1-IN3 per colour (active LOW), ENA / ENB for brightness
CHANNELS = {
    "green (GPIO 17 -> IN1 -> OUT1)": 17,
    "blue  (GPIO 27 -> IN2 -> OUT2)": 27,
    "red   (GPIO 22 -> IN3 -> OUT3)": 22,
}
SECONDS = 3

ena = PWMOutputDevice(5, frequency=500, initial_value=1)  # OUT1 / OUT2 enabled, full
enb = PWMOutputDevice(6, frequency=500, initial_value=1)  # OUT3 / OUT4 enabled, full
pins = {
    name: DigitalOutputDevice(pin, initial_value=True) for name, pin in CHANNELS.items()
}  # all OFF

try:
    while True:
        for name, pin in pins.items():
            print(f"on: {name}", flush=True)
            pin.off()  # LOW = this channel ON
            sleep(SECONDS)
            pin.on()
        print("all off", flush=True)
        sleep(1)

except KeyboardInterrupt:
    print("\nstopped")

finally:
    for device in (ena, enb, *pins.values()):
        device.close()
