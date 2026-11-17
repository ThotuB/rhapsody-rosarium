# museum controller

The museum installation's app: plays the pianos on the installation's speakers and shows them in two
windows, an interface (map and stations) and a projector view. A Wails v3 app: a Go core with React
in the windows.

## Run

Needs the server running (`cd ../server && go run .`) and something playing, e.g. `cd ../node-sim && uv run simulate.py`.
For the projector's keyboard to light up, also the transcriber, see [transcriber/README.md](transcriber/README.md).

```sh
wails3 dev                  # development, with hot reload
wails3 build                # production build, to bin/museum
./bin/museum                # flags below
```

`build/linux/install-desktop.sh` adds the museum to GNOME's app grid, for the current user, so the dock shows
its rose icon (`build/appicon.svg`, rendered to `appicon.png`) rather than the default Wails one. Run it again after
moving the checkout, or with `--remove` to take it away.

The installation's fixed settings live in `config/museum-bulbs.json`, read relative to the folder the app runs from
(`client-museum/` with `wails3 dev` and `task run`). Without the file it uses the defaults below.

```json
{
    "server": "http://localhost:8080",
    "lights": "auto",
    "strips": ["piano-2", "piano-5", null, "piano-4"]
}
```

- `server`: the rhapsody rosarium server
- `lights`: the board driving the led strips, as for `-lights` below
- `strips`: the piano each strip follows, bulb 1 first, using the ids in `server/pianos.json`. `null` (or `""`)
  leaves a bulb off; a list that's missing or empty follows the server's first pianos. A piano id the server
  doesn't know is logged at startup, the bulb stays off

Flags override the file:

- `-config config/museum-bulbs.json`: another settings file
- `-server http://localhost:8080`: the server
- `-tune piano-2`: tune in at startup
- `-headless`: no windows, logs what plays every 5 s
- `-sink null`: play nowhere (for testing without sound), `device` is the sound card
- `-for 70s`: with `-headless`, stop after that long
- `-lights auto`: the board driving the led strips, the first one plugged in. Or a port (`/dev/ttyACM0`), `log` to only log what it would send, or `off`. Without a board the app runs as usual and keeps looking for one
- `-strips piano-2,piano-5,,piano-4`: which piano each strip follows, in strip order; an empty slot leaves that strip off (default: the server's first pianos). Piano ids are the ones in `server/pianos.json`

On Windows the Go side needs cgo, with MSYS2's ucrt64 gcc (from PowerShell; the path is where MSYS2 is installed on the team's Windows laptop):

```powershell
cd client-museum\frontend
npm.cmd ci
npm.cmd run build
cd ..
$env:Path = "D:\Program Files\MSYSK\ucrt64\bin;$env:Path"
$env:CC = "gcc.exe"; $env:CXX = "g++.exe"; $env:CGO_ENABLED = "1"
go run . -lights off
```

`go run .` embeds the frontend's last build, so rebuild it after changing the frontend, and close the old museum
app first.

## Projector

With a second display, both windows start fullscreen: the museum window on the laptop's own screen
(the `eDP` one), the projector window on the other. With a single screen both stay normal windows.
The projector has no controls and ignores mouse and touch; everything is done from the museum window.

It opens on the video at `media/intro.mp4` (relative to the folder the app runs from, or `-video <path>`). It's in
the repository through Git LFS (`git lfs install` once, then `git lfs pull` if a clone has only the pointer file),
and served from disk rather than embedded in the binary. On Linux the web view needs GStreamer's H.264 decoder
(`sudo dnf install gstreamer1-plugin-openh264` on Fedora).

The video waits until it's started with the play button under the museum window's title, next to mute and skip to
the map, and plays with sound. When it ends it fades into the map of the pianos, with an 88 key keyboard along the
bottom that lights up the notes of whatever the museum plays (with the transcriber running). On the map the
play button replays the video. If the video can't be loaded, skip still opens the map. The tuned piano keeps
playing during the video: mute the video, or stop the piano, to hear only one. The map needs internet for its
tiles.

## Led strips

Up to 6 single colour 12 V strips on a GroundStudio Magma Splash (STM32F411) running `lights-board/lights-board.ino`.
The same sketch also runs on a GroundStudio Carbon V3 (ESP32) or an Arduino Uno / Nano, with each board's pins
picked when it's compiled. The strips hang off L298N boards, wiring in the sketch's header and in `hardware.md`.
12 V must never touch the board's pins: they're 3.3 V, and that's how the first Carbon V3 died.

To flash the Magma Splash:

1. Arduino IDE, File → Preferences → Additional boards manager URLs:
   `https://github.com/stm32duino/BoardManagerFiles/raw/main/package_stmicroelectronics_index.json`, then install
   "STM32 MCU based boards" in the Boards Manager
2. Tools: board "Generic STM32F4 series", board part number "BlackPill F411CE (HSE 25Mhz)", USB support
   "CDC (generic 'Serial' supersede U(S)ART)"
3. Open `lights-board/lights-board.ino`, then Sketch → Export Compiled Binary (into `lights-board/build/`, ignored by git)
4. Unplug the strips' 12 V supply: with it on, dfu mode doesn't connect over USB (the kernel logs
   "device not accepting address, error -71"). Then upload mode: hold BOOT0, tap NRST, release BOOT0. `sudo ./dfu-util -l` (in `lights-board/`) lists it as
   `[0483:df11] … "@Internal Flash …"`
5. Flash: `lights-board/flash.sh` (asks for sudo). It finds the Magma in dfu mode by itself, refuses a binary older
   than the sketch, and waits for the board to come back as `/dev/ttyACM0`. By hand, naming the device: other
   things can show up in dfu mode too (a Framework laptop's camera does), and flashing the wrong one can brick it.
   Its serial number is in the `-l` list:

   ```sh
   cd lights-board
   sudo ./dfu-util -d 0483:df11 -S <serial> -a 0 -s 0x08000000:leave -D build/STMicroelectronics.stm32.GenF4/lights-board.ino.bin
   ```

   "Device's firmware is corrupt" at the start is normal for dfu mode. `dfu-util` here is a static Linux x86-64
   build, a copy of the one the Arduino IDE installs.

To upload to a Carbon V3 instead: install "esp32" by Espressif in the Boards Manager, pick "ESP32 Dev Module" and
its port (`/dev/ttyUSB0`), and upload from the IDE.

The board fades and pulses the strips by itself; the app sends a text line per change, which can be typed by hand
in the IDE's serial monitor (115200 baud, line ending "Newline") to try the wiring:

- `L 0 255 500`: strip 0 fades to full over 500 ms
- `P 0 0.5`: strip 0 pulses every 2 s, `P 0 0` stops
- `?`: answers `READY 6`

What the bulbs show (`internal/bulbs`), each following its piano from `config/museum-bulbs.json`:

- the piano the museum is tuned in to: on, following the loudness of what plays (measured against the loudest
  recent sound, so a quiet microphone lights it as fully as a loud one; never below 20%, so it stays visibly on
  between notes)
- other pianos someone is playing right now: a slow pulse, every 4 s, at a quarter brightness
- the rest: off

Tuning in to another piano moves the "on" to its bulb. The constants at the top of `internal/bulbs/bulbs.go` tune the feel.

## Layout

Only the app's entry point is Go at the root: `main.go` embeds `frontend/dist`, and Go's embed can't reach
outside the package's own folder, so that package has to sit above `frontend/`. Wails builds and runs it from
there too (`wails3 dev`, `go run .`). Everything else is a package in `internal/`, Go's folder for packages only
this module can import.

- `main.go`: flags, audio output, the two windows, pushing `state`, `level`, `spectrum` and `playback` events to them
- `museumservice.go`: what the windows can call (`State`, `Tune`, `Stop`, `SetLED`, the intro controls). It stays
  next to `main.go` because the frontend's generated bindings are named after its package
- `config/museum-bulbs.json`, `internal/config`: the installation's settings, overridden by flags
- `internal/windows`: around the windows: fullscreen on the right screens, serving the projector's opening video
- `internal/link`: the connection to the server (`/api/state`, segment downloads)
- `internal/player`: gapless playback of one piano, the live-only rule, fades and pauses at missing windows, a short cushion against late segments. no sound card or windows, so it's tested on its own (`go test ./internal/...`)
- `internal/audio`: the sound card (miniaudio through `malgo`, no system dev packages needed) or a silent stand-in
- `internal/museum`: the core: polls the server, decides what to play, feeds the player
- `internal/lights`: the led strips' board over USB serial: waits for it to be ready, sends what changed, sends everything again after a reset or replug
- `internal/bulbs`: what the strips show, from the museum's state
- `transcriber/`: the Python helper that recognises the notes the projector's keyboard shows
- `lights-board/`: the sketch for the strips' board (Magma Splash, Carbon V3 or Uno / Nano), `flash.sh` and `dfu-util` to flash the Magma
- `frontend/`: React + TypeScript + Tailwind. Both windows load `src/main.tsx`, which picks the window by its url
  (`?view=projector`). `src/museum/` is the museum window, `src/projector/` the projector, `src/shared/` what both
  use: the map, and `useMuseum.ts`, which talks to the Go side. `PianoMap`, `time.ts` and the css are copied from
  `client-web` for now
