# web client

The public page: a map of the city's pianos, and a radio to tune in to one and hear it live, a few seconds behind
the person playing it. While tuned in, the rose in the panel follows the music. React + TypeScript + Tailwind, with
the map from maplibre (OpenFreeMap's tiles, so it needs internet).

## Run

Needs the server running (`cd ../server && go run .`) and something playing, e.g. `cd ../node-sim && uv run simulate.py`.

```sh
npm install
npm run dev          # http://localhost:5173, /api is proxied to the server on localhost:8080
npm run build        # type checks, then builds to dist/
npm run preview      # serves dist/, with the same proxy
npm run lint         # oxlint
npm run format       # prettier, also npm run format:check
```

The dev server listens on the network too, so a phone on the same wifi can open `http://<laptop's ip>:5173`.
The page calls `/api` on its own address, so whatever serves `dist/` has to pass `/api` on to the server the way
the dev server does; the server doesn't serve the page itself.

## How it plays

- `usePianos.ts` polls `GET /api/state` every second
- `radio.ts` is the audio engine, outside React: it downloads and decodes the segments ahead, and schedules them
  back to back on one `AudioContext`, so they join without a gap. It starts a couple of segments behind the newest,
  and leaves a short pause where a segment never arrived (see `continues` in `api.ts`)
- `useListening.ts` tells the server which piano this tab is tuned in to, every 5 s, so the piano's light knows
  someone is listening

## Layout

- `main.tsx`, `App.tsx`: the entry point, the map next to the panel
- `components/PianoMap.tsx`: the map, a rose per piano, which opens while it's tuned in
- `components/Panel.tsx`: the list of pianos, what's playing, play and stop
- `components/Visualizer.tsx`: the rose following the music
- `api.ts`: the server's types and calls
- `time.ts`: which segments are recent enough to play, "x min ago"

`client-museum` has copies of `PianoMap`, `time.ts` and the css.
