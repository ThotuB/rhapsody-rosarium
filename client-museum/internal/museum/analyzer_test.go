package museum

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Exercise the HTTP contract without binding local sockets.
type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestAnalyzerValidatesAndSortsNotes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"chord", `{"duration":2,"notes":[{"pitch":64,"start":1,"duration":0.5,"velocity":80},{"pitch":60,"start":0,"duration":1,"velocity":90}]}`, true},
		{"silence", `{"duration":2,"notes":[]}`, true},
		{"invalid pitch", `{"duration":2,"notes":[{"pitch":999,"start":0,"duration":1,"velocity":80}]}`, false},
		{"invalid timing", `{"duration":2,"notes":[{"pitch":60,"start":0,"duration":10,"velocity":80}]}`, false},
		{"malformed", `oops`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAnalyzer()
			a.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.Header.Get("Content-Type") != "audio/wav" {
					t.Error("wrong upload request")
				}
				return testResponse(http.StatusOK, tc.body), nil
			})
			notes, message := a.analyze(context.Background(), []byte("wav"))
			if (message == "") != tc.valid {
				t.Fatalf("unexpected result: %q", message)
			}
			if len(notes) == 2 && notes[0].Pitch != 60 {
				t.Fatal("notes not sorted by start")
			}
		})
	}
}

func TestAnalyzerFailureHasCooldown(t *testing.T) {
	calls := 0
	a := newAnalyzer()
	a.client.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return testResponse(http.StatusConflict, "busy"), nil
	})
	for i := 0; i < 2; i++ {
		if _, message := a.analyze(context.Background(), nil); message == "" {
			t.Fatal("missing failure status")
		}
	}
	if calls != 1 {
		t.Fatalf("cooldown should prevent repeated requests, got %d", calls)
	}
}
