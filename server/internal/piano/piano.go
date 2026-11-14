// Package piano holds what the server knows about a piano: its entry in pianos.json and its state.
package piano

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"
)

const (
	playingFor   = 30 * time.Second // a segment this recent means someone is playing
	offlineAfter = 45 * time.Second // nodes heartbeat every ~10 s
)

// A piano's state in /api/state. broken and gone are only ever set by hand in pianos.json.
const (
	StatePlaying = "playing" // a segment arrived in the last playingFor
	StateSilent  = "silent"  // the node is connected, nobody is playing
	StateOffline = "offline" // the node stopped reporting
	StateNew     = "new"     // the node has never connected
	StateBroken  = "broken"
	StateGone    = "gone"
)

// StateOf ranks manual states first, then whether the node is reachable, then whether it is being played.
// Zero times mean never.
func StateOf(manual string, lastSeen, lastPlayed, now time.Time) string {
	switch {
	case manual != "":
		return manual
	case lastSeen.IsZero():
		return StateNew
	case now.Sub(lastSeen) > offlineAfter:
		return StateOffline
	case !lastPlayed.IsZero() && now.Sub(lastPlayed) < playingFor:
		return StatePlaying
	default:
		return StateSilent
	}
}

// ids end up in file paths and urls
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// Config is one entry of pianos.json, the fixed set of pianos the server knows about.
type Config struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Token string  `json:"token"`
	// set by hand, "broken" or "gone" win over whatever the node reports
	Status string `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

func LoadConfig(path string) ([]Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pianos []Config
	if err := json.Unmarshal(raw, &pianos); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(pianos) == 0 {
		return nil, fmt.Errorf("%s: no pianos", path)
	}
	seen := map[string]bool{}
	for _, p := range pianos {
		switch {
		case !idPattern.MatchString(p.ID):
			return nil, fmt.Errorf("%s: invalid piano id %q (lowercase letters, digits, - and _)", path, p.ID)
		case seen[p.ID]:
			return nil, fmt.Errorf("%s: duplicate piano id %q", path, p.ID)
		case p.Token == "":
			return nil, fmt.Errorf("%s: piano %q has no token", path, p.ID)
		case p.Status != "" && p.Status != StateBroken && p.Status != StateGone:
			return nil, fmt.Errorf("%s: piano %q has status %q, can only be set to %q or %q", path, p.ID, p.Status, StateBroken, StateGone)
		}
		seen[p.ID] = true
	}
	return pianos, nil
}
