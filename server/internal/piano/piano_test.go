package piano

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateOf(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	var never time.Time
	for _, c := range []struct {
		name, manual string
		seen, played time.Time
		want         string
	}{
		{"never connected", "", never, never, StateNew},
		{"connected, never played", "", ago(5 * time.Second), never, StateSilent},
		{"playing", "", ago(2 * time.Second), ago(10 * time.Second), StatePlaying},
		{"stopped playing", "", ago(2 * time.Second), ago(time.Minute), StateSilent},
		{"node went away", "", ago(time.Minute), ago(time.Minute), StateOffline},
		{"broken wins over playing", StateBroken, ago(time.Second), ago(time.Second), StateBroken},
		{"gone wins over never connected", StateGone, never, never, StateGone},
	} {
		if got := StateOf(c.manual, c.seen, c.played, now); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestConfigStatus(t *testing.T) {
	write := func(status string) string {
		path := filepath.Join(t.TempDir(), "pianos.json")
		os.WriteFile(path, []byte(`[{"id":"a","name":"A","token":"t","status":"`+status+`"}]`), 0o644)
		return path
	}
	if _, err := LoadConfig(write("gone")); err != nil {
		t.Errorf("gone should be allowed: %v", err)
	}
	if _, err := LoadConfig(write("playing")); err == nil {
		t.Error("automatic states can't be set by hand")
	}
}
