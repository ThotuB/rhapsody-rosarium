
"""Purple LED brightness controlled by live listener count."""

import argparse
import threading
import time

import requests
from gpiozero import DigitalOutputDevice, PWMOutputDevice


# Brightness configuration
MIN_BRIGHTNESS = 0
MAX_LISTENERS = 3
POLL_INTERVAL = 2.0
FADE_SPEED = 0.6  # Brightness units per second
UPDATE_INTERVAL = 0.05


def brightness_for(listeners):
    """Map listener count to brightness (0.0 to 1.0)."""
    listeners = max(0, min(listeners, MAX_LISTENERS))

    return MIN_BRIGHTNESS + (
        (1.0 - MIN_BRIGHTNESS)
        * listeners / MAX_LISTENERS
    )


def poll_listeners(args, stop, state, lock):
    """Fetch the audience count from the Go server."""
    session = requests.Session()

    while not stop.is_set():
        try:
            response = session.post(
                f"{args.server}/heartbeat",
                headers={
                    "Authorization": f"Bearer {args.token}"
                },
                data={"node": args.id},
                timeout=5,
            )

            response.raise_for_status()
            data = response.json()

            listeners = int(data["listeners"])
            museum = bool(data.get("museum", False))

            with lock:
                state["listeners"] = listeners
                state["target"] = brightness_for(listeners)

            print(
                f"Listeners: {listeners} | "
                f"Museum: {museum} | "
                f"Target brightness: "
                f"{brightness_for(listeners):.0%}",
                flush=True
            )

        except (
            requests.RequestException,
            ValueError,
            KeyError,
            TypeError,
        ) as exc:
            print(f"Heartbeat failed: {exc}", flush=True)

            # Fade to minimum brightness on connection loss.
            with lock:
                state["target"] = MIN_BRIGHTNESS

        stop.wait(POLL_INTERVAL)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--id", required=True)
    parser.add_argument("--server", required=True)
    parser.add_argument("--token", required=True)
    args = parser.parse_args()

    args.server = args.server.rstrip("/")

    stop = threading.Event()
    lock = threading.Lock()

    state = {
        "listeners": 0,
        "target": MIN_BRIGHTNESS,
    }

    # Existing L298N wiring
    ena = DigitalOutputDevice(5, initial_value=False)
    enb = DigitalOutputDevice(6, initial_value=False)

    red = DigitalOutputDevice(17, initial_value=True)
    green = DigitalOutputDevice(27, initial_value=True)
    blue = DigitalOutputDevice(22, initial_value=True)

    # Enable PWM brightness control
    ena.close()
    enb.close()

    ena = PWMOutputDevice(5, frequency=500, initial_value=0)
    enb = PWMOutputDevice(6, frequency=500, initial_value=0)

    current_brightness = 0.0

    try:
        # Fixed purple: red + blue
        red.off()
        green.on()
        blue.off()

        worker = threading.Thread(
            target=poll_listeners,
            args=(args, stop, state, lock),
            daemon=True,
        )
        worker.start()

        print("Listener-controlled purple LED started")

        last_time = time.monotonic()

        while not stop.is_set():
            now = time.monotonic()
            dt = now - last_time
            last_time = now

            with lock:
                target = state["target"]

            # Gradually approach the new brightness.
            max_change = FADE_SPEED * dt

            difference = target - current_brightness

            if abs(difference) <= max_change:
                current_brightness = target
            elif difference > 0:
                current_brightness += max_change
            else:
                current_brightness -= max_change

            current_brightness = max(
                0.0, min(1.0, current_brightness)
            )

            ena.value = current_brightness
            enb.value = current_brightness

            time.sleep(UPDATE_INTERVAL)

    except KeyboardInterrupt:
        print("\nStopping LED controller")

    finally:
        stop.set()

        ena.value = 0
        enb.value = 0

        ena.close()
        enb.close()
        red.close()
        green.close()
        blue.close()


if __name__ == "__main__":
    main()
