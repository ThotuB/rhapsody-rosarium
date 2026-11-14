package player

import (
	"slices"
	"time"

	"rhapsody-rosarium/museum/internal/link"
)

// how far back tuning in reaches, so listening always feels live (the same rule as the web client)
const liveWindow = time.Minute

// Playable is what may be played of a piano right now: its audio from the last minute, oldest
// first, and only while its node is connected. now is the server's clock from the same state.
func Playable(p link.Piano, now time.Time) []link.Segment {
	if p.State != "playing" && p.State != "silent" {
		return nil
	}
	var out []link.Segment
	for _, s := range p.Segments {
		if now.Sub(s.At) < liveWindow {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b link.Segment) int { return a.At.Compare(b.At) })
	return out
}

// Continues reports whether next is the window straight after prev, so the two can play back to back.
// A skipped number (a window that never arrived) or a reset (the node restarted) means a gap.
// Without numbers there's no way to tell, so they're assumed to continue.
func Continues(prev, next link.Segment) bool {
	if prev.Seq == nil || next.Seq == nil {
		return true
	}
	return *next.Seq == *prev.Seq+1
}
