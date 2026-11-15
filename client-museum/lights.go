package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"rhapsody-rosarium/museum/internal/lights"
	"rhapsody-rosarium/museum/internal/link"
	"rhapsody-rosarium/museum/internal/museum"
)

// startLights drives the led strips from the museum's state until ctx is done, and returns a
// channel closed once they're switched off. port is as for lights.New, or "off".
func startLights(ctx context.Context, port string, strips []string, core *museum.Core) <-chan struct{} {
	done := make(chan struct{})
	if port == "off" {
		close(done)
		return done
	}
	l := lights.New(port)
	go func() {
		l.Run(ctx)
		close(done)
	}()
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		checked := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				v := core.View()
				if !checked && len(v.Pianos) > 0 { // once the server's pianos are known
					warnUnknownPianos(strips, v.Pianos)
					checked = true
				}
				showOnLights(l, v, strips)
			}
		}
	}()
	return done
}

// showOnLights decides what the strips show. A placeholder until that's decided: strip i follows
// the i-th piano of strips (by default the server's first pianos) like that piano's own led at
// the instrument, off while nobody listens and pulsing faster with every listener.
func showOnLights(l *lights.Lights, v museum.View, strips []string) {
	if len(strips) == 0 {
		for _, p := range v.Pianos {
			strips = append(strips, p.ID)
		}
	}
	listeners := map[string]int{}
	for _, p := range v.Pianos {
		listeners[p.ID] = p.Listeners
	}
	for i := range lights.Strips {
		n := 0
		if i < len(strips) {
			n = listeners[strips[i]]
		}
		s := lights.Strip{Fade: time.Second} // fades on and off
		if n > 0 {
			// the same speeds as node-piano/lights.py: +0.25 pulses per second per listener, at most 2.5
			s.Level, s.Pulses = 255, min(0.25*float64(n), 2.5)
		}
		l.Set(i, s)
	}
}

// warnUnknownPianos logs the strips set to follow a piano the server doesn't have, e.g. a typo in
// museum-bulbs.json: those stay dark, which would otherwise look like a wiring fault.
func warnUnknownPianos(strips []string, pianos []link.Piano) {
	known := map[string]bool{}
	for _, p := range pianos {
		known[p.ID] = true
	}
	for i, id := range strips {
		if id != "" && !known[id] {
			slog.Warn("lights: no such piano on the server, the strip stays off", "bulb", i+1, "piano", id)
		}
	}
}

// pianoList splits "piano-2,piano-3" into ids, nil for "". An empty slot ("piano-2,,piano-4") stays
// in place as "", so the strips after it keep their pianos and that strip stays off.
func pianoList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var ids []string
	for id := range strings.SplitSeq(s, ",") {
		ids = append(ids, strings.TrimSpace(id))
	}
	return ids
}
