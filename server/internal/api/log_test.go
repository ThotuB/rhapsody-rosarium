package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogRequests(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	dir := t.TempDir()
	h := LogRequests((&API{store: newTestStore(t, dir, 5)}).Handler())
	os.WriteFile(filepath.Join(dir, "a", "1-5.wav"), bytes.Repeat([]byte("x"), 1000), 0o644)

	get(h, "/api/state")
	upload(h, "a", "nope", wav()) // rejected
	req := httptest.NewRequest("GET", "/api/audio/a/1-5.wav", nil)
	req.Header.Set("Range", "bytes=0-99")
	req.Header.Set("CF-Connecting-IP", "203.0.113.7") // as the cloudflare tunnel sends it
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// the wrapper mustn't get in the way of partial responses
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Errorf("range through the logger: got %d with %d bytes, want 206 with 100", rec.Code, rec.Body.Len())
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	for _, want := range []string{
		"method=GET path=/api/state status=200",
		"method=POST path=/api/segments status=401",
		"method=GET path=/api/audio/a/1-5.wav status=206 bytes=100",
	} {
		found := false
		for _, line := range lines {
			found = found || strings.Contains(line, want)
		}
		if !found {
			t.Errorf("no log line with %q in:\n%s", want, logs.String())
		}
	}
	if !strings.Contains(lines[len(lines)-1], "from=203.0.113.7") {
		t.Errorf("behind the tunnel, the visitor's address should be logged: %s", lines[len(lines)-1])
	}
}
