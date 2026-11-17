# piano node

Runs on the Raspberry Pi at a piano. It records the microphone in 2 s segments and uploads them to the server, and
pulses an RGB LED strip at the piano: faster the more people listen, in the colour the museum picked for it.

## Run

On the Pi, with `ffmpeg` (`sudo apt install ffmpeg`) and [uv](https://docs.astral.sh/uv/):

```sh
uv sync
uv run app.py --id piano-2 --server http://<server's ip>:8080/api --token dev
```

- `--id`, `--token`: the piano's id and token from `server/pianos.json`
- `--server`: the server's address, ending in `/api`
- `--gain-db 14`: how much to turn the microphone up. A limiter keeps the loudest notes just under full scale, so
  more gain mostly squashes the music harder instead of making it louder (see `record.py`)

Python 3.12 (in `.python-version`): `lgpio`, the pin driver, has ready-built wheels up to 3.12, so `uv sync` doesn't
compile anything on the Pi.

The microphone is `MIC` at the top of `record.py`, an ALSA name (`arecord -L` lists them). Without the LED's pins,
e.g. on a laptop, it logs that and records anyway.

## What it does

- `record.py`: ffmpeg records the microphone into `segments/`, one numbered mp3 per 2 s. The next number is kept in
  `segments/state.json`, so numbers keep counting up across restarts
- `app.py`: uploads the finished segments in order and deletes them. A segment older than 8 s is dropped
  instead: listeners only play recent ones. Every 2 s it sends a heartbeat, whose reply says how many people listen
  and the LED's colour
- `lights.py`: the LED, in a thread of its own. Off with nobody listening, a pulse every 4 s with one listener,
  faster with each extra one
- `api.py`: the two calls to the server

## LED wiring

A 12 V RGB strip through an L298N: GPIO 22 → IN3 (red), GPIO 17 → IN1 (green), GPIO 27 → IN2 (blue), active low,
with GPIO 5 → ENA and GPIO 6 → ENB held on. `test/channels.py` lights one colour at a time to check which wire is
which, and `test/led.py` fades the strip purple. Stop `app.py` first, it holds the pins.

`.lgd-nfy0` in the folder the node runs from is a named pipe that `lgpio` makes when it starts. It's left behind
when the node stops, is ignored by git, and is safe to delete.
