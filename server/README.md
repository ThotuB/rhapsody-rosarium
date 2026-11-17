# server

The middle of Rhapsody Rosarium. The piano nodes (`node-piano` on a Raspberry Pi, or `node-sim`) upload 2 s
segments of what the microphone hears; the clients (`client-web`, `client-museum`) poll which pianos there are and
what they played, and download the segments to play them back to back. It also keeps track of who is listening to
which piano, and the colour the museum set for each piano's light, which the nodes hear back in their heartbeats.

## Run

Needs Go and `ffmpeg` on the PATH: every upload is converted with it, to mono 48 kHz wav.

```sh
go run .          # or make run
make build        # to bin/rhapsody-rosarium
make test         # go test ./...
make vet          # gofmt -l and go vet
```

Flags:

- `-addr :8080`: where it listens
- `-config pianos.json`: the pianos it knows about, see below
- `-data data`: where segments are stored, a folder per piano (ignored by git)
- `-keep 30`: segments kept per piano, older ones are deleted
- `-ffmpeg ffmpeg`: the ffmpeg binary
- `-log-requests=false`: stop logging every request

Segments on disk are picked up again after a restart. Listeners and the lights' colours are only kept in memory.

## pianos.json

The fixed list of pianos, read at startup:

```json
{
  "id": "piano-1",
  "name": "Parcul Central",
  "lat": 45.751795,
  "lng": 21.220549,
  "token": "dev",
  "status": "gone",
  "note": "taken away in spring"
}
```

- `id`: lowercase letters, digits, `-` and `_`, it ends up in file names and urls
- `token`: what that piano's node sends as `Authorization: Bearer <token>`
- `status` (optional): `broken` or `gone`, set by hand, shown instead of what the node reports
- `note` (optional): shown with the piano, e.g. why it's broken

Otherwise a piano's state is worked out from its node: `playing` (a segment in the last 30 s), `silent` (the node is
there, nobody plays), `offline` (nothing heard for 45 s) or `new` (never connected).

## API

All under `/api`:

| | | |
|---|---|---|
| `POST /segments` | node, token | multipart: `node` (piano id), `audio` (anything ffmpeg reads), `seq` (optional, the node's window number, from 0). 422 if the audio can't be decoded |
| `POST /heartbeat` | node, token | form: `node`. Answers `{"listeners": 2, "museum": true, "led": "#ff00ff"}` |
| `GET /state` | clients | every piano with its state, last seen and played, and its segments' urls, oldest first |
| `GET /audio/{piano}/{file}` | clients | one segment, wav |
| `POST /listening` | clients | form: `client` (an id the client made up), `piano` (empty once it stops), `museum=1` from the museum. Repeated every few seconds; a client is forgotten 15 s after its last one |
| `POST /pianos/{piano}/led` | museum | form: `color`, `#rrggbb`. The node picks it up with its next heartbeat |

Listening and the lights need no token: it's a demo.

## Layout

- `main.go`: flags, starting the server
- `internal/api`: the endpoints above, request logging
- `internal/store`: the segments on disk, converting uploads, when each node was last seen, the lights' colours
- `internal/piano`: `pianos.json` and how a piano's state is worked out
- `internal/listen`: who is listening to which piano
