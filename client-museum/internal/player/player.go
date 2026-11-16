// Package player plays one piano at a time, gaplessly: segments are decoded and appended to a
// queue of samples that the sound card reads from in order. Where a window is missing, the audio
// fades out, pauses briefly and fades back in, rather than gluing two unrelated moments together.
//
// It knows nothing about sound cards or windows, so it can be tested on its own.
package player

import (
	"math"
	"slices"
	"sync"
	"time"

	"rhapsody-rosarium/museum/internal/link"
)

const (
	Rate      = 48000          // samples per second, what the server stores
	startBack = 2              // tuning in starts this many segments before the newest, see Status.Behind
	fade      = Rate * 3 / 100 // 30 ms, faded out and in around a gap
	pause     = Rate / 4       // 250 ms of silence where a window is missing
	// after silence, playback waits until this much audio is lined up, so a segment arriving a
	// little late doesn't cut out the sound. it waits at most this long, then plays what it has
	cushion = Rate * 3
)

type queued struct {
	seg           link.Segment
	start         int
	end           int // where its samples end in buf
	notes         []Note
	analysisError string
}

// Note is a detected note timed relative to its audio segment.
type Note struct {
	Pitch    int     `json:"pitch"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	Velocity float64 `json:"velocity"`
}

// Player is safe to use from several goroutines: the sound card's callback calls Read while the
// core calls Tune, Want and Enqueue.
type Player struct {
	mu        sync.Mutex
	piano     string        // the tuned piano, "" when off
	buf       []int16       // samples lined up to play
	pos       int           // the next sample of buf to play
	queued    []queued      // segments with samples still in buf, in order
	heard     time.Time     // the newest segment queued since tuning in, so nothing plays twice
	last      *link.Segment // the last segment queued, to tell whether the next one joins it
	started   bool          // played audio since tuning in, so running dry is an underrun
	starving  bool          // ran dry and hasn't recovered yet
	holding   bool          // silent until a cushion of audio is lined up
	held      int           // samples of silence spent holding with something lined up
	underruns int
	level     float64 // loudness of the last samples read, 0 to 1
}

func New() *Player {
	return &Player{holding: true}
}

// Tune switches to another piano ("" to stop). What was playing fades out quickly instead of cutting off.
func (p *Player) Tune(piano string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var tail []int16
	if !p.holding { // while holding nothing is audible yet, so there's nothing to fade
		rest := p.buf[p.pos:]
		tail = slices.Clone(rest[:min(len(rest), fade)])
		fadeOut(tail)
	}
	p.buf, p.pos, p.queued = tail, 0, nil
	p.piano = piano
	p.heard, p.last = time.Time{}, nil
	p.started, p.starving = false, false
	p.held = 0
}

func (p *Player) Tuned() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.piano
}

// Want returns the segments to fetch next for the piano, from what's playable of it: a few back
// from the newest right after tuning in, afterwards only what's newer than everything queued.
func (p *Player) Want(piano string, playable []link.Segment) []link.Segment {
	p.mu.Lock()
	defer p.mu.Unlock()
	if piano != p.piano || piano == "" {
		return nil
	}
	sorted := slices.SortedFunc(slices.Values(playable), func(a, b link.Segment) int { return a.At.Compare(b.At) })
	if p.heard.IsZero() {
		return sorted[max(0, len(sorted)-startBack):]
	}
	var fresh []link.Segment
	for _, s := range sorted {
		if s.At.After(p.heard) {
			fresh = append(fresh, s)
		}
	}
	return fresh
}

// Enqueue lines up a segment's samples after what's already queued. It's ignored if the piano was
// retuned meanwhile, or if the segment isn't newer than what's queued.
func (p *Player) Enqueue(piano string, seg link.Segment, samples []int16) {
	p.EnqueueAnalyzed(piano, seg, samples, nil, "")
}

// EnqueueAnalyzed keeps note timing attached to the actual speaker samples.
func (p *Player) EnqueueAnalyzed(piano string, seg link.Segment, samples []int16, notes []Note, analysisError string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if piano != p.piano || piano == "" || (!p.heard.IsZero() && !seg.At.After(p.heard)) {
		return
	}
	p.compact()

	empty := p.pos == len(p.buf) || p.last == nil // nothing playing, or only the last piano fading out
	gap := p.last != nil && !Continues(*p.last, seg)
	if gap && !empty {
		fadeOut(p.buf[max(p.pos, len(p.buf)-fade):])
	}
	if gap {
		p.buf = append(p.buf, make([]int16, pause)...)
	}
	start := len(p.buf)
	p.buf = append(p.buf, samples...)
	if gap || empty {
		// after a gap, and when starting or restarting from silence, so it doesn't click
		fadeIn(p.buf[start:min(len(p.buf), start+fade)])
	}

	p.queued = append(p.queued, queued{seg: seg, start: start, end: len(p.buf), notes: slices.Clone(notes), analysisError: analysisError})
	p.heard = seg.At
	p.last = &seg
}

// Read fills out with the next samples, and with silence once nothing is lined up.
// It's called from the sound card's callback, so it only copies and never blocks for long.
func (p *Player) Read(out []int16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.holding {
		left := len(p.buf) - p.pos
		if left < cushion && p.held < cushion {
			clear(out)
			if left > 0 {
				p.held += len(out)
			}
			p.level = 0
			return
		}
		p.holding = false
	}

	n := copy(out, p.buf[p.pos:])
	p.pos += n
	clear(out[n:])

	for len(p.queued) > 0 && p.queued[0].end <= p.pos {
		p.queued = p.queued[1:]
	}
	if n > 0 {
		p.started, p.starving = true, false
	}
	if n < len(out) {
		p.holding, p.held = true, 0 // ran dry: wait for a new cushion before playing again
		if p.started && p.piano != "" && !p.starving {
			p.starving = true
			p.underruns++
		}
	}

	var sum float64
	for _, s := range out {
		v := float64(s) / math.MaxInt16
		sum += v * v
	}
	if len(out) > 0 {
		p.level = math.Sqrt(sum / float64(len(out)))
	}
}

// Status is a snapshot of what the player is doing.
type Status struct {
	Piano         string        // "" when off
	Segment       *link.Segment // the segment playing now
	Buffered      time.Duration // audio lined up and not played yet
	Behind        time.Duration // how long ago the piano played what's playing now, 0 before anything is queued
	Holding       bool          // silent, waiting for a cushion of audio before playing
	Underruns     int           // times it ran dry while playing, since starting
	Level         float64       // loudness, 0 to 1
	Position      float64       // seconds into the current segment; negative in gap silence
	Duration      float64
	Notes         []Note
	AnalysisError string
}

func (p *Player) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := Status{
		Piano:     p.piano,
		Buffered:  time.Duration(len(p.buf)-p.pos) * time.Second / Rate,
		Holding:   p.holding,
		Underruns: p.underruns,
		Level:     p.level,
	}
	if len(p.queued) > 0 {
		seg := p.queued[0].seg
		st.Segment = &seg
		st.Position = float64(p.pos-p.queued[0].start) / Rate
		st.Duration = float64(p.queued[0].end-p.queued[0].start) / Rate
		st.Notes = p.queued[0].notes
		st.AnalysisError = p.queued[0].analysisError
	}
	if !p.heard.IsZero() {
		// the newest segment queued ends with what the piano played just before uploading it, which
		// reached the server at heard; what's playing now is everything lined up before that.
		// leaves out the upload itself, and assumes this clock agrees with the server's
		st.Behind = time.Since(p.heard) + st.Buffered
	}
	return st
}

// compact drops samples already played, so buf doesn't grow forever. It keeps the last fftSize
// played, for Spectrum.
func (p *Player) compact() {
	if p.pos < Rate*5 {
		return
	}
	drop := p.pos - fftSize
	p.buf = slices.Delete(p.buf, 0, drop)
	for i := range p.queued {
		p.queued[i].start -= drop
		p.queued[i].end -= drop
	}
	p.pos = fftSize
}

func fadeOut(s []int16) {
	for i := range s {
		s[i] = int16(float64(s[i]) * float64(len(s)-1-i) / float64(len(s)))
	}
}

func fadeIn(s []int16) {
	for i := range s {
		s[i] = int16(float64(s[i]) * float64(i) / float64(len(s)))
	}
}
