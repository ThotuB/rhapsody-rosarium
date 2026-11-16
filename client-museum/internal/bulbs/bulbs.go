// Package bulbs decides what the museum's led strips show, from the museum's state and the
// loudness of what plays.
package bulbs

import (
	"context"
	"log/slog"
	"math"
	"time"

	"rhapsody-rosarium/museum/internal/lights"
	"rhapsody-rosarium/museum/internal/link"
	"rhapsody-rosarium/museum/internal/museum"
)

// What the bulbs show, each following one piano (see config/museum-bulbs.json):
//   - the piano the museum is tuned in to: on, following the loudness of what plays
//   - other pianos someone is playing right now: a slow pulse at a quarter brightness
//   - the rest: off
//
// Tuning in to another piano moves the "on" to its bulb, and the old one goes back to pulsing or off.
const (
	bulbTick = 40 * time.Millisecond // how often the bulbs are updated: often enough to follow notes

	tunedFloor = 0.2 // the tuned bulb is never darker than this, so it stays visibly on between notes
	// loudness rises quickly and falls back more slowly, so the bulb follows notes without flickering
	attack, release = 0.6, 0.15
	// loudness is measured against the loudest recent sound, which fades by half in about this long:
	// a quiet microphone then lights the bulb as fully as a loud one
	peakHalfLife = 8 * time.Second
	minPeak      = 0.02 // quieter than this is near silence (a mic's own noise), which isn't scaled up into flicker
	// music rarely drops far below its recent peak, so the differences are exaggerated: at 2, a passage
	// at 70% of the peak's loudness lights the bulb at about half, instead of looking nearly steady
	contrast = 2

	playingLevel  = 0.25 // other playing pianos: a quarter brightness
	playingPulses = 0.25 // pulses per second: one every 4 s
)

// Start drives the led strips from the museum's state and the loudness of what plays until
// ctx is done, and returns a channel closed once they're switched off. port is as for lights.New,
// or "off".
func Start(ctx context.Context, port string, strips []string, core *museum.Core, level func() float64) <-chan struct{} {
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
		tick := time.NewTicker(bulbTick)
		defer tick.Stop()
		b := &bulbs{strips: strips}
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
				b.show(l, v, level())
			}
		}
	}()
	return done
}

// setter is what the bulbs are shown on: *lights.Lights, or a fake in tests.
type setter interface {
	Set(i int, s lights.Strip)
}

// bulbs decides what the strips show, see the rules above. show is called every bulbTick.
type bulbs struct {
	strips []string // the piano each strip follows; empty: the server's first pianos
	loud   float64  // smoothed loudness of what plays, 0 to 1
	peak   float64  // the loudest recent sound, slowly fading
}

var peakDecay = math.Pow(0.5, float64(bulbTick)/float64(peakHalfLife)) // per tick

func (b *bulbs) show(out setter, v museum.View, level float64) {
	k := release
	if level > b.loud {
		k = attack
	}
	b.loud += (level - b.loud) * k
	b.peak = max(b.loud, b.peak*peakDecay, minPeak)

	ids := b.strips
	if len(ids) == 0 {
		for _, p := range v.Pianos {
			ids = append(ids, p.ID)
		}
	}
	playing := map[string]bool{}
	for _, p := range v.Pianos {
		playing[p.ID] = p.State == "playing"
	}
	for i := range lights.Strips {
		id := ""
		if i < len(ids) {
			id = ids[i]
		}
		switch {
		case id != "" && id == v.Tuned:
			bright := tunedFloor + (1-tunedFloor)*math.Pow(min(1, b.loud/b.peak), contrast)
			out.Set(i, lights.Strip{Level: level255(bright), Fade: bulbTick})
		case id != "" && playing[id]:
			out.Set(i, lights.Strip{Level: level255(playingLevel), Fade: time.Second, Pulses: playingPulses})
		default:
			out.Set(i, lights.Strip{Fade: time.Second}) // fades out
		}
	}
}

// level255 turns a brightness from 0 to 1 into the firmware's 0-255. The firmware corrects for how
// eyes see brightness, so 0.25 looks like a quarter.
func level255(brightness float64) uint8 {
	return uint8(math.Round(min(max(brightness, 0), 1) * 255))
}

// warnUnknownPianos logs the strips set to follow a piano the server doesn't have, e.g. a typo in
// config/museum-bulbs.json: those stay dark, which would otherwise look like a wiring fault.
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
