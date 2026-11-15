// Package api is the server's HTTP interface, for the piano nodes and the clients.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rhapsody-rosarium/internal/listen"
	"rhapsody-rosarium/internal/store"
)

const maxUpload = 32 << 20

type API struct {
	store     *store.Store
	listeners listen.Table
}

func New(s *store.Store) *API {
	return &API{store: s}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/segments", a.postSegment)
	mux.HandleFunc("POST /api/heartbeat", a.postHeartbeat)
	mux.HandleFunc("POST /api/listening", a.postListening)
	mux.HandleFunc("POST /api/pianos/{piano}/led", a.postLED)
	mux.HandleFunc("GET /api/state", a.getState)
	mux.HandleFunc("GET /api/audio/{piano}/{file}", a.getAudio)
	return mux
}

func bearer(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return token
}

// postSegment takes a multipart form with `node` (piano id), `audio` (any format ffmpeg reads)
// and optionally `seq` (the node's window number, counting up from 0 per recording session),
// authorised by that piano's token.
func (a *API) postSegment(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if !a.store.ValidToken(token) { // before reading the body
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "upload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "expected a multipart form", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	id := r.FormValue("node")
	if !a.store.Authorize(id, token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	seq := int64(store.NoSeq)
	if v := r.FormValue("seq"); v != "" {
		var err error
		if seq, err = strconv.ParseInt(v, 10, 64); err != nil || seq < 0 {
			http.Error(w, "seq must be a whole number from 0 up", http.StatusBadRequest)
			return
		}
	}
	file, _, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "missing audio field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	seg, err := a.store.Ingest(r.Context(), id, seq, file)
	switch {
	case errors.Is(err, store.ErrBadAudio):
		slog.Warn("rejected segment", "node", id, "err", err)
		http.Error(w, "could not decode audio", http.StatusUnprocessableEntity)
	case errors.Is(err, context.Canceled): // the node hung up, it will retry
		slog.Info("upload abandoned", "node", id)
	case err != nil:
		slog.Error("storing segment failed", "node", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		slog.Info("segment", "node", id, "url", seg.URL)
		writeJSON(w, seg)
	}
}

// nodeReply is what a node hears back from a heartbeat: its audience, and the colour the museum set for its light.
type nodeReply struct {
	listen.Audience
	LED string `json:"led"` // #rrggbb
}

// postHeartbeat lets a node say it's alive between uploads, so a piano nobody plays shows as silent, not offline.
// Takes a form with `node`, authorised by that piano's token. Answers with who is listening to the piano and the
// colour of its light, {"listeners": 2, "museum": true, "led": "#ff00ff"}, so the node can react to them.
func (a *API) postHeartbeat(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if !a.store.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	id := r.FormValue("node")
	if !a.store.Authorize(id, token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a.store.Seen(id)
	writeJSON(w, nodeReply{Audience: a.listeners.Of(id), LED: a.store.LED(id)})
}

// postLED sets the colour of the light at a piano: a form with `color`, #rrggbb. The node picks it up with its
// next heartbeat. Meant for the museum; no token, like the rest of the demo's listener side.
func (a *API) postLED(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	switch err := a.store.SetLED(r.PathValue("piano"), r.FormValue("color")); {
	case errors.Is(err, store.ErrUnknownPiano):
		http.Error(w, "unknown piano", http.StatusNotFound)
	case errors.Is(err, store.ErrBadColour):
		http.Error(w, "color must be #rrggbb", http.StatusBadRequest)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// postListening is a client saying what it's tuned in to, again every few seconds while it listens.
// Takes a form with `client` (any id the client made up, the same on every call), `piano` (empty once it
// stopped) and `museum=1` from the museum installation. No token: listeners are anonymous.
func (a *API) postListening(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	client, id := r.FormValue("client"), r.FormValue("piano")
	if client == "" || len(client) > 64 {
		http.Error(w, "client must be an id of 1 to 64 characters", http.StatusBadRequest)
		return
	}
	if id != "" && !a.store.Has(id) {
		http.Error(w, "unknown piano", http.StatusNotFound)
		return
	}
	a.listeners.Mark(client, id, r.FormValue("museum") == "1")
	w.WriteHeader(http.StatusNoContent)
}

type stateResponse struct {
	Now    time.Time         `json:"now"`
	Pianos []store.PianoView `json:"pianos"`
}

func (a *API) getState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	pianos, listening := a.store.Snapshot(), a.listeners.All()
	for i := range pianos {
		pianos[i].Audience = listening[pianos[i].ID]
	}
	writeJSON(w, stateResponse{Now: time.Now(), Pianos: pianos})
}

func (a *API) getAudio(w http.ResponseWriter, r *http.Request) {
	path, ok := a.store.SegmentPath(r.PathValue("piano"), r.PathValue("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, path) // handles Range requests, which safari needs
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
