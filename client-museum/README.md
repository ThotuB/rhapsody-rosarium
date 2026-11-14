# museum controller

The museum installation's app: plays the pianos on the installation's speakers and shows them in two
windows, an interface (map and stations) and a projector view. A Wails v3 app: a Go core with React
in the windows.

## Run

Needs the server running (`cd ../server && go run .`) and something playing, e.g. `cd ../node-sim && uv run simulate.py`.

```sh
wails3 dev                  # development, with hot reload
wails3 build                # production build, to bin/museum
./bin/museum                # flags below
```

- `-server http://localhost:8080`: the server
- `-tune piano-2`: tune in at startup
- `-headless`: no windows, logs what plays every 5 s
- `-sink null`: play nowhere (for testing without sound), `device` is the sound card
- `-for 70s`: with `-headless`, stop after that long
- `-lights auto`: the board driving the led strips, the first one plugged in. Or a port (`/dev/ttyUSB0`), `log` to only log what it would send, or `off`. Without a board the app runs as usual and keeps looking for one
- `-strips piano-2,piano-3`: which piano each strip follows, in strip order (default: the server's first pianos)

The projector window goes fullscreen on a second display when there is one, otherwise it stays a normal window.

## Led strips

Up to 6 single colour 12 V strips on a GroundStudio Carbon V3 (ESP32) running `firmware/lights/lights.ino`. The
same sketch also runs on an Arduino Uno / Nano or a GroundStudio Magma Splash, with each board's pins picked when
it's compiled. The strips hang off L298N boards, wiring in the sketch's header and in `hardware.md`.

To upload to the Carbon V3: in the Arduino IDE's Boards Manager install "esp32" by Espressif, pick the board
"ESP32 Dev Module" and its port (`/dev/ttyUSB0`), open the sketch and upload.

The board fades and pulses the strips by itself; the app sends a text line per change, which can be typed by hand
in the IDE's serial monitor (115200 baud, line ending "Newline") to try the wiring:

- `L 0 255 500`: strip 0 fades to full over 500 ms
- `P 0 0.5`: strip 0 pulses every 2 s, `P 0 0` stops
- `?`: answers `READY 6`

What the strips show is a placeholder for now (`lights.go`): each follows one piano like that piano's own led,
off while nobody listens and pulsing faster with every listener.

## Layout

- `main.go`: flags, audio output, the two windows, pushing `state` and `level` events to them
- `museumservice.go`: what the windows can call: `State`, `Tune`, `Stop`
- `internal/link`: the connection to the server (`/api/state`, segment downloads)
- `internal/player`: gapless playback of one piano, the live-only rule, fades and pauses at missing windows, a short cushion against late segments. no sound card or windows, so it's tested on its own (`go test ./internal/...`)
- `internal/audio`: the sound card (miniaudio through `malgo`, no system dev packages needed) or a silent stand-in
- `internal/museum`: the core: polls the server, decides what to play, feeds the player
- `internal/lights`: the led strips' board over USB serial: waits for it to be ready, sends what changed, sends everything again after a reset or replug
- `lights.go`: what the strips show, from the museum's state
- `firmware/lights/`: the sketch for the strips' board (Carbon V3, Uno / Nano or Magma Splash)
- `frontend/`: React + TypeScript + Tailwind. `PianoMap`, `time.ts` and the css are copied from `client-web` for now
