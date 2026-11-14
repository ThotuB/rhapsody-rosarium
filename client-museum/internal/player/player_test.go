package player

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"
	"time"

	"rhapsody-rosarium/museum/internal/link"
)

var t0 = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func seg(i int, seq *int64) link.Segment {
	return link.Segment{URL: "/s", At: t0.Add(time.Duration(i) * time.Second), Seq: seq}
}

func num(n int64) *int64 { return &n }

// a recognisable signal: distinct values, never zero, so fades and pauses show up
func samples(n int, base int16) []int16 {
	s := make([]int16, n)
	for i := range s {
		s[i] = base + int16(i%100)
	}
	return s
}

// read everything lined up, in uneven chunks like a sound card would ask for. the silence while
// it holds for a cushion isn't part of the audio, so it's left out
func drain(p *Player) []int16 {
	left := func() int {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.buf) - p.pos
	}
	var out []int16
	for {
		before := left()
		if before == 0 {
			return out
		}
		chunk := make([]int16, min(333, before))
		p.Read(chunk)
		out = append(out, chunk[:before-left()]...) // only what was actually played
	}
}

func wav(rate uint32, channels uint16, data []int16) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	body := new(bytes.Buffer)
	body.WriteString("WAVE")
	// a LIST chunk before fmt, like ffmpeg writes, to check the chunk walking
	body.WriteString("LIST")
	binary.Write(body, le, uint32(5))
	body.WriteString("INFOx\x00") // odd size, padded
	body.WriteString("fmt ")
	binary.Write(body, le, uint32(16))
	binary.Write(body, le, struct {
		Format, Channels uint16
		Rate, ByteRate   uint32
		Align, Bits      uint16
	}{1, channels, rate, rate * 2 * uint32(channels), 2 * channels, 16})
	body.WriteString("data")
	binary.Write(body, le, uint32(2*len(data)))
	binary.Write(body, le, data)
	b.WriteString("RIFF")
	binary.Write(&b, le, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

func TestDecodeWAV(t *testing.T) {
	want := samples(1000, 7)
	got, err := DecodeWAV(wav(Rate, 1, want))
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %d samples, err %v", len(got), err)
	}
	for name, b := range map[string][]byte{
		"44.1 kHz": wav(44100, 1, want),
		"stereo":   wav(Rate, 2, want),
		"not wav":  []byte("hello"),
	} {
		if _, err := DecodeWAV(b); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestConsecutiveSegmentsAreSeamless(t *testing.T) {
	p := New()
	p.Tune("a")
	a, b, c := samples(5000, 100), samples(5000, 300), samples(5000, 500)
	p.Enqueue("a", seg(0, num(0)), a)
	p.Enqueue("a", seg(1, num(1)), b)
	p.Enqueue("a", seg(2, num(2)), c)
	got := drain(p)

	want := slices.Concat(a, b, c)
	fadeIn(want[:fade]) // only the very start fades in, after tuning in
	if !slices.Equal(got, want) {
		t.Fatalf("segments didn't play back to back: got %d samples, want %d", len(got), len(want))
	}
}

func TestWithoutWindowNumbersSegmentsJoin(t *testing.T) {
	p := New()
	p.Tune("a")
	a, b := samples(4000, 100), samples(4000, 300)
	p.Enqueue("a", seg(0, nil), a)
	p.Enqueue("a", seg(1, nil), b)
	if got := drain(p); len(got) != len(a)+len(b) {
		t.Errorf("got %d samples, want %d with no pause", len(got), len(a)+len(b))
	}
}

func TestMissingWindowFadesAndPauses(t *testing.T) {
	p := New()
	p.Tune("a")
	a, b := samples(5000, 100), samples(5000, 300)
	p.Enqueue("a", seg(0, num(4)), a)
	p.Enqueue("a", seg(1, num(7)), b) // 5 and 6 never arrived
	got := drain(p)

	if len(got) != len(a)+pause+len(b) {
		t.Fatalf("got %d samples, want %d (with a %d sample pause)", len(got), len(a)+pause+len(b), pause)
	}
	if got[len(a)-1] != 0 || got[len(a)-fade] == 0 {
		t.Error("the first segment should fade out to silence at its end")
	}
	if slices.ContainsFunc(got[len(a):len(a)+pause], func(s int16) bool { return s != 0 }) {
		t.Error("the pause should be silent")
	}
	start := len(a) + pause
	if got[start] != 0 || got[start+fade] != b[fade] {
		t.Error("the second segment should fade in, then play unchanged")
	}

	// a reset to 0 (the node restarted) is a gap too
	p.Enqueue("a", seg(2, num(0)), samples(5000, 500))
	if got := drain(p); len(got) != pause+5000 {
		t.Errorf("after a restart: got %d samples, want a pause first", len(got))
	}
}

func TestTuningClearsTheQueue(t *testing.T) {
	p := New()
	p.Tune("a")
	p.Enqueue("a", seg(0, num(0)), samples(20000, 100))
	p.Read(make([]int16, 1000))
	p.Tune("b")
	if got := p.Status().Buffered; got > fade*time.Second/Rate {
		t.Errorf("after retuning, %v of the old piano is still lined up", got)
	}
	// a late download for the old piano is ignored
	p.Enqueue("a", seg(1, num(1)), samples(20000, 100))
	if got := p.Status().Buffered; got > fade*time.Second/Rate {
		t.Errorf("a segment for the old piano was queued: %v lined up", got)
	}

	// retuned while still holding for a cushion: nothing of the old piano was audible, so none of
	// it plays; the new piano starts with its own fade in
	p.Tune("c")
	if got := p.Status().Buffered; got != 0 {
		t.Errorf("retuned while holding, but %v of the old piano is still lined up", got)
	}
	c := samples(cushion, 500)
	p.Enqueue("c", seg(2, num(0)), c)
	got := drain(p)
	want := slices.Clone(c)
	fadeIn(want[:fade])
	if !slices.Equal(got, want) {
		t.Error("the new piano should start with its own faded in samples, nothing of the old one")
	}
}

func TestWantStartsAFewBackThenOnlyNewer(t *testing.T) {
	p := New()
	p.Tune("a")
	playable := []link.Segment{seg(0, nil), seg(1, nil), seg(2, nil), seg(3, nil), seg(4, nil)}
	first := p.Want("a", playable)
	if len(first) != startBack || !first[0].At.Equal(playable[len(playable)-startBack].At) {
		t.Fatalf("tuning in should start %d back from the newest: %v", startBack, first)
	}
	for _, s := range first {
		p.Enqueue("a", s, samples(100, 1))
	}
	if again := p.Want("a", playable); len(again) != 0 {
		t.Errorf("nothing new, but wants %v", again)
	}
	if next := p.Want("a", append(playable, seg(5, nil))); len(next) != 1 || !next[0].At.Equal(seg(5, nil).At) {
		t.Errorf("should want only the new segment: %v", next)
	}
	if other := p.Want("b", playable); other != nil {
		t.Errorf("wants segments for a piano that isn't tuned: %v", other)
	}
}

func TestUnderrunsAreCountedOncePerDrySpell(t *testing.T) {
	p := New()
	p.Tune("a")
	p.Read(make([]int16, 100)) // nothing yet: waiting, not an underrun
	if p.Status().Underruns != 0 {
		t.Fatal("waiting for the first segment isn't an underrun")
	}
	p.Enqueue("a", seg(0, nil), samples(cushion+500, 100))
	p.Read(make([]int16, cushion)) // a full cushion: plays right away
	p.Read(make([]int16, 400))
	p.Read(make([]int16, 400)) // runs dry
	p.Read(make([]int16, 400)) // still dry, same spell
	if got := p.Status().Underruns; got != 1 {
		t.Errorf("got %d underruns, want 1", got)
	}
	out := make([]int16, 400)
	p.Read(out)
	if slices.ContainsFunc(out, func(s int16) bool { return s != 0 }) {
		t.Error("with nothing lined up, it should play silence")
	}
}

func TestHoldsUntilACushionIsLinedUp(t *testing.T) {
	p := New()
	p.Tune("a")
	p.Enqueue("a", seg(0, nil), samples(Rate, 100)) // 1 s, less than the cushion
	out := make([]int16, Rate/10)
	p.Read(out)
	if slices.ContainsFunc(out, func(s int16) bool { return s != 0 }) || !p.Status().Holding {
		t.Fatal("with less than a cushion lined up, it should hold, silent")
	}
	// enough arrives: it plays straight away
	p.Enqueue("a", seg(1, nil), samples(cushion, 300))
	p.Read(out)
	if p.Status().Holding || !slices.ContainsFunc(out, func(s int16) bool { return s != 0 }) {
		t.Error("with a cushion lined up, it should play")
	}

	// nothing more arrives: it doesn't hold forever, it plays what it has after a cushion's time
	q := New()
	q.Tune("a")
	q.Enqueue("a", seg(0, nil), samples(Rate, 100))
	for range cushion / len(out) {
		q.Read(out)
	}
	q.Read(out)
	if q.Status().Holding {
		t.Error("it should stop holding after waiting a cushion's time")
	}
}

func TestPlayableIsLiveOnly(t *testing.T) {
	now := t0.Add(2 * time.Minute)
	piano := link.Piano{State: "playing", Segments: []link.Segment{
		{At: now.Add(-90 * time.Second)}, {At: now.Add(-20 * time.Second)}, {At: now.Add(-50 * time.Second)},
	}}
	got := Playable(piano, now)
	if len(got) != 2 || !got[0].At.Before(got[1].At) {
		t.Errorf("want the last minute, oldest first: %v", got)
	}
	for _, state := range []string{"offline", "broken", "gone", "new"} {
		piano.State = state
		if got := Playable(piano, now); got != nil {
			t.Errorf("%s: nothing should be playable, got %v", state, got)
		}
	}
}
