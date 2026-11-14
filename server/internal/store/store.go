// Package store keeps the pianos' audio segments on disk and tracks when each node was last seen.
package store

import (
	"bytes"
	"cmp"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"rhapsody-rosarium/internal/piano"
)

const (
	convertTimeout = 10 * time.Second
	// the web client plays at this rate too: a browser resampling each segment on its own
	// leaves a small glitch at every join, so everything is stored at the rate it's played at
	sampleRate = "48000"
)

// stored segments are <unix ms>-<seq>.wav, or <unix ms>.wav when the node sent no window number;
// uploads in progress are upload-*.
// wav because mp3 and aac pad every file with silence, which the client would hear between segments.
var segmentFile = regexp.MustCompile(`^(\d+)(?:-(\d+))?\.wav$`)

// ErrBadAudio means the upload could not be decoded, as opposed to a server side failure.
var ErrBadAudio = errors.New("undecodable audio")

// NoSeq is the window number of a segment whose node didn't send one.
const NoSeq = -1

type segment struct {
	ms  int64 // when it was received, unix ms
	seq int64 // the node's window number, or NoSeq
}

func (g segment) file() string {
	if g.seq == NoSeq {
		return fmt.Sprintf("%d.wav", g.ms)
	}
	return fmt.Sprintf("%d-%d.wav", g.ms, g.seq)
}

type pianoState struct {
	cfg      piano.Config
	segs     []segment // oldest first
	lastSeen time.Time // last heartbeat or upload
}

// Store keeps the last `keep` segments of every piano as wav files under dir/<piano id>/.
// The set of pianos is fixed at construction, so only the segment lists need the lock.
type Store struct {
	dir    string
	keep   int
	ffmpeg string
	Now    func() time.Time // the clock, replaced in tests

	mu     sync.Mutex
	pianos map[string]*pianoState
	order  []string
}

func NewStore(dir string, keep int, ffmpeg string, cfgs []piano.Config) (*Store, error) {
	s := &Store{dir: dir, keep: keep, ffmpeg: ffmpeg, Now: time.Now, pianos: map[string]*pianoState{}}
	for _, c := range cfgs {
		p := &pianoState{cfg: c}
		pd := filepath.Join(dir, c.ID)
		if err := os.MkdirAll(pd, 0o755); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(pd)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if m := segmentFile.FindStringSubmatch(e.Name()); m != nil {
				g := segment{seq: NoSeq}
				g.ms, _ = strconv.ParseInt(m[1], 10, 64)
				if m[2] != "" {
					g.seq, _ = strconv.ParseInt(m[2], 10, 64)
				}
				p.segs = append(p.segs, g)
			} else if strings.HasPrefix(e.Name(), "upload-") {
				os.Remove(filepath.Join(pd, e.Name())) // left over from a crash
			}
		}
		slices.SortFunc(p.segs, func(a, b segment) int { return cmp.Compare(a.ms, b.ms) })
		s.removeFiles(c.ID, s.prune(p))
		if n := len(p.segs); n > 0 {
			p.lastSeen = time.UnixMilli(p.segs[n-1].ms) // best guess after a restart, until the next heartbeat
		}
		s.pianos[c.ID] = p
		s.order = append(s.order, c.ID)
	}
	return s, nil
}

// Authorize reports whether the piano exists and the token is its node's.
func (s *Store) Authorize(id, token string) bool {
	p, ok := s.pianos[id]
	return ok && tokenEqual(token, p.cfg.Token)
}

// SegmentPath returns where a stored segment of the piano lives on disk, if file is a valid segment name.
// The file itself may no longer exist.
func (s *Store) SegmentPath(id, file string) (string, bool) {
	if _, ok := s.pianos[id]; !ok || !segmentFile.MatchString(file) {
		return "", false
	}
	return filepath.Join(s.dir, id, file), true
}

// ValidToken reports whether the token belongs to any piano.
func (s *Store) ValidToken(token string) bool {
	ok := false
	for _, p := range s.pianos {
		ok = tokenEqual(token, p.cfg.Token) || ok
	}
	return ok
}

// Seen records a heartbeat from the piano's node.
func (s *Store) Seen(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.pianos[id]; ok {
		p.lastSeen = s.Now()
	}
}

func tokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Ingest converts the uploaded audio to mono wav and adds it as the newest segment of the piano.
// seq is the node's window number, or NoSeq if it didn't send one.
func (s *Store) Ingest(ctx context.Context, id string, seq int64, audio io.Reader) (SegmentView, error) {
	p, ok := s.pianos[id]
	if !ok {
		return SegmentView{}, fmt.Errorf("unknown piano %q", id)
	}
	dir := filepath.Join(s.dir, id)

	// a real file, not a pipe: mp4/aac recordings from safari can't be decoded from stdin
	in, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		return SegmentView{}, err
	}
	defer os.Remove(in.Name())
	_, err = io.Copy(in, audio)
	if cerr := in.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return SegmentView{}, err
	}

	out := in.Name() + ".wav"
	defer os.Remove(out)
	if err := s.convert(ctx, in.Name(), out); err != nil {
		return SegmentView{}, err
	}

	g, old, err := s.commit(p, dir, out, seq)
	if err != nil {
		return SegmentView{}, err
	}
	s.removeFiles(id, old)
	return segmentView(id, g), nil
}

func (s *Store) convert(ctx context.Context, in, out string) error {
	ctx, cancel := context.WithTimeout(ctx, convertTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-nostdin", "-v", "error", "-y", "-i", in,
		"-vn", "-ac", "1", "-ar", sampleRate, "-c:a", "pcm_s16le", "-f", "wav", out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return fmt.Errorf("ffmpeg: %w", ctx.Err())
	case errors.As(err, &exit):
		return fmt.Errorf("%w: %s", ErrBadAudio, strings.TrimSpace(stderr.String()))
	default:
		return err
	}
}

// commit moves the converted file into place under a fresh timestamp and returns the segments that fell out of the buffer.
func (s *Store) commit(p *pianoState, dir, converted string, seq int64) (segment, []segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	g := segment{ms: now.UnixMilli(), seq: seq}
	if n := len(p.segs); n > 0 && g.ms <= p.segs[n-1].ms {
		g.ms = p.segs[n-1].ms + 1
	}
	if err := os.Rename(converted, filepath.Join(dir, g.file())); err != nil {
		return segment{}, nil, err
	}
	p.segs = append(p.segs, g)
	p.lastSeen = now // an upload is a heartbeat too
	return g, s.prune(p), nil
}

// prune drops the oldest segments beyond keep from the list; the caller deletes their files.
func (s *Store) prune(p *pianoState) []segment {
	if len(p.segs) <= s.keep {
		return nil
	}
	n := len(p.segs) - s.keep
	old := slices.Clone(p.segs[:n])
	p.segs = slices.Clone(p.segs[n:])
	return old
}

func (s *Store) removeFiles(id string, segs []segment) {
	for _, g := range segs {
		os.Remove(filepath.Join(s.dir, id, g.file()))
	}
}

type SegmentView struct {
	URL string    `json:"url"`
	At  time.Time `json:"at"`
	// the node's window number: consecutive numbers are consecutive audio. null if the node sent none
	Seq *int64 `json:"seq"`
}

type PianoView struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Lat        float64       `json:"lat"`
	Lng        float64       `json:"lng"`
	State      string        `json:"state"`
	Note       *string       `json:"note"` // null unless set in pianos.json
	LastSeen   *time.Time    `json:"lastSeen"`
	LastPlayed *time.Time    `json:"lastPlayed"`
	Segments   []SegmentView `json:"segments"` // oldest first
}

func segmentView(id string, g segment) SegmentView {
	v := SegmentView{URL: fmt.Sprintf("/api/audio/%s/%s", id, g.file()), At: time.UnixMilli(g.ms)}
	if g.seq != NoSeq {
		seq := g.seq
		v.Seq = &seq
	}
	return v
}

// Snapshot returns every piano, in config order, with its buffered segments.
func (s *Store) Snapshot() []PianoView {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	views := make([]PianoView, 0, len(s.order))
	for _, id := range s.order {
		p := s.pianos[id]
		v := PianoView{ID: id, Name: p.cfg.Name, Lat: p.cfg.Lat, Lng: p.cfg.Lng, Segments: make([]SegmentView, 0, len(p.segs))}
		if p.cfg.Note != "" {
			note := p.cfg.Note
			v.Note = &note
		}
		for _, g := range p.segs {
			v.Segments = append(v.Segments, segmentView(id, g))
		}
		var lastPlayed time.Time
		if n := len(v.Segments); n > 0 {
			lastPlayed = v.Segments[n-1].At
			v.LastPlayed = &lastPlayed
		}
		if !p.lastSeen.IsZero() {
			seen := p.lastSeen
			v.LastSeen = &seen
		}
		v.State = piano.StateOf(p.cfg.Status, p.lastSeen, lastPlayed, now)
		views = append(views, v)
	}
	return views
}
