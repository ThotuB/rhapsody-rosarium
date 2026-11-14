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

The projector window goes fullscreen on a second display when there is one, otherwise it stays a normal window.

## Layout

- `main.go`: flags, audio output, the two windows, pushing `state` and `level` events to them
- `museumservice.go`: what the windows can call: `State`, `Tune`, `Stop`
- `internal/link`: the connection to the server (`/api/state`, segment downloads)
- `internal/player`: gapless playback of one piano, the live-only rule, fades and pauses at missing windows, a short cushion against late segments. no sound card or windows, so it's tested on its own (`go test ./internal/...`)
- `internal/audio`: the sound card (miniaudio through `malgo`, no system dev packages needed) or a silent stand-in
- `internal/museum`: the core: polls the server, decides what to play, feeds the player
- `frontend/`: React + TypeScript + Tailwind. `PianoMap`, `time.ts` and the css are copied from `client-web` for now
