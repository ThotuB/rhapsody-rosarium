package link

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetLED(t *testing.T) {
	var path, colour string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, colour = r.URL.Path, r.FormValue("color")
		if colour == "bad" {
			http.Error(w, "color must be #rrggbb", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := New(srv.URL)

	if err := c.SetLED(context.Background(), "piano-2", "#00ff80"); err != nil {
		t.Fatal(err)
	}
	if path != "/api/pianos/piano-2/led" || colour != "#00ff80" {
		t.Errorf("posted %q with color %q", path, colour)
	}
	if err := c.SetLED(context.Background(), "piano-2", "bad"); err == nil {
		t.Error("a rejected colour should be an error, with the server's reason")
	} else if got := err.Error(); got != "POST /api/pianos/piano-2/led: 400 Bad Request color must be #rrggbb" {
		t.Errorf("error: %q", got)
	}
}
