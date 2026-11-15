package main

import (
	"slices"
	"testing"
	"time"

	"rhapsody-rosarium/museum/internal/lights"
	"rhapsody-rosarium/museum/internal/link"
	"rhapsody-rosarium/museum/internal/museum"
)

func TestPianoList(t *testing.T) {
	for in, want := range map[string][]string{
		"":                          nil,
		"piano-2":                   {"piano-2"},
		"piano-2, piano-5,piano-7 ": {"piano-2", "piano-5", "piano-7"},
		"piano-2,piano-5,,piano-4":  {"piano-2", "piano-5", "", "piano-4"}, // bulb 3 off, bulb 4 keeps its piano
		",piano-5":                  {"", "piano-5"},
	} {
		if got := pianoList(in); !slices.Equal(got, want) {
			t.Errorf("pianoList(%q) = %q, want %q", in, got, want)
		}
	}
}

// shown records what each strip was last set to.
type shown map[int]lights.Strip

func (s shown) Set(i int, strip lights.Strip) { s[i] = strip }

func view(tuned string, states map[string]string) museum.View {
	v := museum.View{Tuned: tuned}
	for _, id := range []string{"a", "b", "c"} {
		v.Pianos = append(v.Pianos, link.Piano{ID: id, State: states[id]})
	}
	return v
}

// run shows the same view and loudness for a while, as the bulbs' ticker would.
func run(b *bulbs, out shown, v museum.View, level float64, d time.Duration) {
	for range int(d / bulbTick) {
		b.show(out, v, level)
	}
}

var (
	pulsing = lights.Strip{Level: 64, Fade: time.Second, Pulses: playingPulses}
	off     = lights.Strip{Fade: time.Second}
)

func TestBulbRules(t *testing.T) {
	b := &bulbs{strips: []string{"a", "b", "", "c"}}
	out := shown{}
	states := map[string]string{"a": "playing", "b": "playing", "c": "silent"}

	run(b, out, view("a", states), 0.2, 2*time.Second)
	if s := out[0]; s.Pulses != 0 || s.Fade != bulbTick || s.Level < 250 {
		t.Errorf("tuned in to a, with sound: its bulb should be on, near full: %+v", s)
	}
	if out[1] != pulsing {
		t.Errorf("b is playing, not tuned: a slow pulse at a quarter: %+v", out[1])
	}
	if out[2] != off || out[3] != off {
		t.Errorf("an empty slot and a silent piano are off: %+v, %+v", out[2], out[3])
	}
	for i := 4; i < lights.Strips; i++ {
		if out[i] != off {
			t.Errorf("strip %d follows nothing, should be off: %+v", i, out[i])
		}
	}

	run(b, out, view("b", states), 0.2, time.Second) // tuned in to b instead
	if out[0] != pulsing {
		t.Errorf("after switching away, a's bulb goes back to pulsing: %+v", out[0])
	}
	if s := out[1]; s.Pulses != 0 || s.Level < 250 {
		t.Errorf("after switching to b, its bulb is on: %+v", s)
	}

	run(b, out, view("b", states), 0, 2*time.Second) // silence on b
	if got, want := out[1].Level, level255(tunedFloor); got != want {
		t.Errorf("in silence the tuned bulb dims to its floor, not off: %d, want %d", got, want)
	}

	states["a"] = "silent" // a stops being played
	run(b, out, view("b", states), 0, time.Second)
	if out[0] != off {
		t.Errorf("a isn't played any more: off: %+v", out[0])
	}

	run(b, out, view("", states), 0, time.Second) // stopped listening
	if out[1] != pulsing {
		t.Errorf("not tuned in any more, b is still playing: pulsing: %+v", out[1])
	}
}

func TestTunedBulbFollowsTheSound(t *testing.T) {
	v := view("a", map[string]string{"a": "playing"})
	for name, loud := range map[string]float64{"loud": 0.25, "quiet microphone": 0.03} {
		b := &bulbs{strips: []string{"a"}}
		out := shown{}
		run(b, out, v, loud, 3*time.Second)
		if out[0].Level < 250 {
			t.Errorf("%s, steady: measured against its own recent peak, near full: %d", name, out[0].Level)
		}
		run(b, out, v, loud/4, 300*time.Millisecond) // a quieter passage
		if l := out[0].Level; l > 160 || l <= level255(tunedFloor) {
			t.Errorf("%s, a quarter as loud: dimmer, but above the floor: %d", name, l)
		}
	}
}
