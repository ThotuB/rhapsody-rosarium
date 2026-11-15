package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"rhapsody-rosarium/internal/listen"
	"rhapsody-rosarium/internal/piano"
	"rhapsody-rosarium/internal/store"
)

var testPianos = []piano.Config{
	{ID: "a", Name: "A", Token: "token-a"},
	{ID: "b", Name: "B", Token: "token-b"},
}

func newTestStore(t *testing.T, dir string, keep int) *store.Store {
	t.Helper()
	s, err := store.NewStore(dir, keep, "ffmpeg", testPianos)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func needFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
}

// a short 440 Hz tone
func wav() []byte {
	const rate, n = 8000, 2400
	le := binary.LittleEndian
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, le, uint32(36+2*n))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, le, struct {
		Size             uint32
		Format, Channels uint16
		Rate, ByteRate   uint32
		Align, Bits      uint16
	}{16, 1, 1, rate, rate * 2, 2, 16})
	b.WriteString("data")
	binary.Write(&b, le, uint32(2*n))
	for i := range n {
		binary.Write(&b, le, int16(8000*math.Sin(2*math.Pi*440*float64(i)/rate)))
	}
	return b.Bytes()
}

func upload(h http.Handler, node, token string, audio []byte) *httptest.ResponseRecorder {
	return uploadSeq(h, node, token, audio, "")
}

// uploadSeq uploads with a window number, or none if seq is empty
func uploadSeq(h http.Handler, node, token string, audio []byte, seq string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("node", node)
	if seq != "" {
		mw.WriteField("seq", seq)
	}
	fw, _ := mw.CreateFormFile("audio", "segment.wav")
	fw.Write(audio)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/segments", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func heartbeat(h http.Handler, node, token string) int {
	return heartbeatReply(h, node, token).Code
}

func heartbeatReply(h http.Handler, node, token string) *httptest.ResponseRecorder {
	var auth []string
	if token != "" {
		auth = []string{"Authorization", "Bearer " + token}
	}
	return postForm(h, "/api/heartbeat", url.Values{"node": {node}}, auth...)
}

func postForm(h http.Handler, path string, form url.Values, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func pianosByID(t *testing.T, h http.Handler) map[string]store.PianoView {
	t.Helper()
	var state stateResponse
	if err := json.Unmarshal(get(h, "/api/state").Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	byID := map[string]store.PianoView{}
	for _, p := range state.Pianos {
		byID[p.ID] = p
	}
	return byID
}

func get(h http.Handler, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadAuth(t *testing.T) {
	h := (&API{store: newTestStore(t, t.TempDir(), 5)}).Handler()
	for name, c := range map[string]struct{ node, token string }{
		"no token":            {"a", ""},
		"wrong token":         {"a", "nope"},
		"other piano's token": {"a", "token-b"},
		"unknown piano":       {"zzz", "token-a"},
	} {
		if rec := upload(h, c.node, c.token, wav()); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
	}
}

func TestUploadAndServe(t *testing.T) {
	needFFmpeg(t)
	h := (&API{store: newTestStore(t, t.TempDir(), 5)}).Handler()

	if rec := upload(h, "a", "token-a", wav()); rec.Code != http.StatusOK {
		t.Fatalf("upload: got %d %s", rec.Code, rec.Body)
	}
	if rec := upload(h, "a", "token-a", []byte("not audio")); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("garbage: got %d, want 422", rec.Code)
	}

	var state stateResponse
	if err := json.Unmarshal(get(h, "/api/state").Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Pianos) != 2 || state.Pianos[0].ID != "a" || len(state.Pianos[0].Segments) != 1 || state.Pianos[0].LastPlayed == nil {
		t.Fatalf("unexpected state: %+v", state)
	}
	if b := state.Pianos[1]; b.LastPlayed != nil || b.Segments == nil || len(b.Segments) != 0 {
		t.Errorf("idle piano should have no segments and no lastPlayed: %+v", b)
	}
	if a, b := state.Pianos[0].State, state.Pianos[1].State; a != piano.StatePlaying || b != piano.StateNew {
		t.Errorf("states: got a=%s b=%s, want playing and new", a, b)
	}

	url := state.Pianos[0].Segments[0].URL
	rec := get(h, url)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "audio/wav" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("RIFF")) {
		t.Fatalf("audio: got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(h, url, "Range", "bytes=0-99"); rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Errorf("range: got %d with %d bytes, want 206 with 100", rec.Code, rec.Body.Len())
	}
}

func TestRingBufferAndRestart(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	h := (&API{store: newTestStore(t, dir, 3)}).Handler()
	for range 5 {
		if rec := upload(h, "a", "token-a", wav()); rec.Code != http.StatusOK {
			t.Fatalf("upload: got %d %s", rec.Code, rec.Body)
		}
	}

	segs := func(h http.Handler) []store.SegmentView {
		var state stateResponse
		json.Unmarshal(get(h, "/api/state").Body.Bytes(), &state)
		return state.Pianos[0].Segments
	}
	before := segs(h)
	if len(before) != 3 {
		t.Fatalf("kept %d segments, want 3", len(before))
	}
	for i := 1; i < len(before); i++ {
		if !before[i].At.After(before[i-1].At) {
			t.Errorf("segments out of order: %v", before)
		}
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "a", "*")); len(files) != 3 {
		t.Errorf("%d files on disk, want 3", len(files))
	}

	restarted := (&API{store: newTestStore(t, dir, 3)}).Handler()
	after := segs(restarted)
	if len(after) != 3 || after[0] != before[0] || after[2] != before[2] {
		t.Errorf("restart changed state:\nbefore %v\nafter  %v", before, after)
	}
	if a := pianosByID(t, restarted)["a"]; a.State == piano.StateNew || a.LastSeen == nil {
		t.Errorf("after a restart a piano with segments should count as seen: %+v", a)
	}

	// a smaller buffer trims on startup, and crash leftovers are cleared
	os.WriteFile(filepath.Join(dir, "a", "upload-123"), []byte("x"), 0o644)
	if got := segs((&API{store: newTestStore(t, dir, 2)}).Handler()); len(got) != 2 || got[1] != before[2] {
		t.Errorf("keep=2 gave %v", got)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "a", "*")); len(files) != 2 {
		t.Errorf("%d files on disk after restart, want 2", len(files))
	}
}

func TestHeartbeat(t *testing.T) {
	broken := piano.Config{ID: "c", Name: "C", Token: "token-c", Status: piano.StateBroken, Note: "out of tune"}
	st, err := store.NewStore(t.TempDir(), 5, "ffmpeg", append(slices.Clone(testPianos), broken))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	st.Now = func() time.Time { return clock }
	h := (&API{store: st}).Handler()

	for name, c := range map[string]struct{ node, token string }{
		"no token":            {"a", ""},
		"other piano's token": {"a", "token-b"},
		"unknown piano":       {"zzz", "token-a"},
	} {
		if code := heartbeat(h, c.node, c.token); code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, code)
		}
	}

	if a := pianosByID(t, h)["a"]; a.State != piano.StateNew || a.LastSeen != nil {
		t.Errorf("before any heartbeat: %+v", a)
	}
	if code := heartbeat(h, "a", "token-a"); code != http.StatusOK {
		t.Fatalf("heartbeat: got %d, want 200", code)
	}
	if a := pianosByID(t, h)["a"]; a.State != piano.StateSilent || a.LastSeen == nil || !a.LastSeen.Equal(clock) {
		t.Errorf("after a heartbeat: %+v", a)
	}

	clock = clock.Add(time.Minute)
	if a := pianosByID(t, h)["a"]; a.State != piano.StateOffline {
		t.Errorf("a minute without heartbeats: got %s, want offline", a.State)
	}

	heartbeat(h, "c", "token-c")
	if c := pianosByID(t, h)["c"]; c.State != piano.StateBroken || c.Note == nil || *c.Note != "out of tune" {
		t.Errorf("manual state should win over heartbeats: %+v", c)
	}
	if !strings.Contains(get(h, "/api/state").Body.String(), `"note":null`) {
		t.Error("pianos without a note should still have the field, as null")
	}
}

func TestAudioRouteRejects(t *testing.T) {
	dir := t.TempDir()
	h := (&API{store: newTestStore(t, dir, 5)}).Handler()
	// files that exist but aren't segments must not be served either
	for _, name := range []string{"1.mp3", "upload-1", "1-x.wav", "1-.wav"} {
		os.WriteFile(filepath.Join(dir, "a", name), []byte("x"), 0o644)
	}
	for _, path := range []string{
		"/api/audio/zzz/1.wav",
		"/api/audio/a/1.mp3",
		"/api/audio/a/upload-1",
		"/api/audio/a/1-x.wav",
		"/api/audio/a/1-.wav",
		"/api/audio/a/..%2F..%2Fetc%2Fpasswd",
		"/api/audio/..%2Fa/1.wav",
		"/api/audio/a/1.wav", // valid name, nothing stored
	} {
		if rec := get(h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, rec.Code)
		}
	}
	// a segment with a window number in its name is served
	os.WriteFile(filepath.Join(dir, "a", "1-5.wav"), []byte("RIFF"), 0o644)
	if rec := get(h, "/api/audio/a/1-5.wav"); rec.Code != http.StatusOK {
		t.Errorf("1-5.wav: got %d, want 200", rec.Code)
	}
}

func TestWindowNumbers(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	h := (&API{store: newTestStore(t, dir, 5)}).Handler()

	for _, bad := range []string{"-1", "x", "1.5"} {
		if rec := uploadSeq(h, "a", "token-a", wav(), bad); rec.Code != http.StatusBadRequest {
			t.Errorf("seq %q: got %d, want 400", bad, rec.Code)
		}
	}
	for _, seq := range []string{"0", "1", "", "7"} {
		if rec := uploadSeq(h, "a", "token-a", wav(), seq); rec.Code != http.StatusOK {
			t.Fatalf("seq %q: got %d %s", seq, rec.Code, rec.Body)
		}
	}

	// compare values, not the pointers inside the views
	seqs := func(h http.Handler) []string {
		var out []string
		for _, s := range pianosByID(t, h)["a"].Segments {
			if s.Seq == nil {
				out = append(out, "null")
			} else {
				out = append(out, strconv.FormatInt(*s.Seq, 10))
			}
		}
		return out
	}
	want := []string{"0", "1", "null", "7"}
	if got := seqs(h); !slices.Equal(got, want) {
		t.Errorf("window numbers: got %v, want %v", got, want)
	}
	if url := pianosByID(t, h)["a"].Segments[1].URL; !strings.HasSuffix(url, "-1.wav") {
		t.Errorf("url %s doesn't carry the window number", url)
	}

	// they survive a restart, since they're part of the file names
	restarted := (&API{store: newTestStore(t, dir, 5)}).Handler()
	if got := seqs(restarted); !slices.Equal(got, want) {
		t.Errorf("after a restart: got %v, want %v", got, want)
	}
}

func TestListening(t *testing.T) {
	a := &API{store: newTestStore(t, t.TempDir(), 5)}
	h := a.Handler()
	listening := func(form url.Values) int { return postForm(h, "/api/listening", form).Code }
	audience := func() listen.Audience {
		var got listen.Audience
		rec := heartbeatReply(h, "a", "token-a")
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("heartbeat reply %q: %v", rec.Body, err)
		}
		return got
	}

	if got := audience(); got.Listeners != 0 || got.Museum {
		t.Errorf("nobody listening yet: %+v", got)
	}
	for name, c := range map[string]struct {
		form url.Values
		want int
	}{
		"no client id":  {url.Values{"piano": {"a"}}, http.StatusBadRequest},
		"unknown piano": {url.Values{"client": {"x"}, "piano": {"zzz"}}, http.StatusNotFound},
	} {
		if code := listening(c.form); code != c.want {
			t.Errorf("%s: got %d, want %d", name, code, c.want)
		}
	}

	listening(url.Values{"client": {"web-1"}, "piano": {"a"}})
	listening(url.Values{"client": {"museum"}, "piano": {"a"}, "museum": {"1"}})
	listening(url.Values{"client": {"web-2"}, "piano": {"b"}})
	if got := audience(); got.Listeners != 2 || !got.Museum {
		t.Errorf("web-1 and the museum on a: %+v", got)
	}
	if p := pianosByID(t, h); p["a"].Listeners != 2 || !p["a"].Museum || p["b"].Listeners != 1 || p["b"].Museum {
		t.Errorf("listeners in the state: a %+v, b %+v", p["a"].Audience, p["b"].Audience)
	}

	listening(url.Values{"client": {"museum"}, "piano": {""}, "museum": {"1"}}) // stopped
	if got := audience(); got.Listeners != 1 || got.Museum {
		t.Errorf("after the museum stopped: %+v", got)
	}

	a.listeners.Now = func() time.Time { return time.Now().Add(time.Minute) }
	if got := audience(); got.Listeners != 0 {
		t.Errorf("a client that stops saying it listens should expire: %+v", got)
	}
}

func TestLED(t *testing.T) {
	h := (&API{store: newTestStore(t, t.TempDir(), 5)}).Handler()
	setLED := func(piano, colour string) int {
		return postForm(h, "/api/pianos/"+piano+"/led", url.Values{"color": {colour}}).Code
	}
	led := func() string {
		var reply nodeReply
		rec := heartbeatReply(h, "a", "token-a")
		if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
			t.Fatalf("heartbeat reply %q: %v", rec.Body, err)
		}
		return reply.LED
	}

	if got := led(); got != store.DefaultLED {
		t.Errorf("before the museum sets one, the default colour: %q", got)
	}
	for name, c := range map[string]struct {
		piano, colour string
		want          int
	}{
		"unknown piano":   {"zzz", "#00ff00", http.StatusNotFound},
		"no colour":       {"a", "", http.StatusBadRequest},
		"not hex":         {"a", "#gg0000", http.StatusBadRequest},
		"too short":       {"a", "#fff", http.StatusBadRequest},
		"a colour's name": {"a", "red", http.StatusBadRequest},
	} {
		if code := setLED(c.piano, c.colour); code != c.want {
			t.Errorf("%s: got %d, want %d", name, code, c.want)
		}
	}

	if code := setLED("a", "#00FF80"); code != http.StatusNoContent {
		t.Fatalf("setting a colour: got %d", code)
	}
	if got := led(); got != "#00ff80" {
		t.Errorf("the node should hear the new colour, lower case: %q", got)
	}
	if p := pianosByID(t, h); p["a"].LED != "#00ff80" || p["b"].LED != store.DefaultLED {
		t.Errorf("colours in the state: a %q, b %q", p["a"].LED, p["b"].LED)
	}
}
