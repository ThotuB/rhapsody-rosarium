package player

import (
	"testing"
)

func TestNotesFollowSamplesAcrossSegmentsAndCompaction(t *testing.T) {
	p := New()
	p.Tune("a")
	first := []Note{{Pitch: 60, Start: 0, Duration: 1, Velocity: 80}}
	second := []Note{{Pitch: 64, Start: 2, Duration: 1, Velocity: 90}}
	p.EnqueueAnalyzed("a", seg(0, num(0)), samples(4*Rate, 100), first, "")
	p.EnqueueAnalyzed("a", seg(1, num(1)), samples(4*Rate, 100), second, "")
	first[0].Pitch = 10 // the caller cannot mutate the queued snapshot
	p.Read(make([]int16, Rate))
	st := p.Status()
	if st.Position != 1 || st.Duration != 4 || st.Notes[0].Pitch != 60 {
		t.Fatalf("wrong first-segment timing/notes: %+v", st)
	}
	p.Read(make([]int16, 3*Rate))
	st = p.Status()
	if st.Position != 0 || st.Notes[0].Pitch != 64 {
		t.Fatalf("notes did not switch at the audio boundary: %+v", st)
	}
	p.Read(make([]int16, 2*Rate))
	p.Enqueue("a", seg(2, num(2)), samples(4*Rate, 100)) // compacts six seconds
	st = p.Status()
	if st.Position != 2 || st.Duration != 4 || st.Notes[0].Pitch != 64 {
		t.Fatalf("compaction moved note timing: %+v", st)
	}
	p.Tune("b")
	if st := p.Status(); st.Segment != nil || len(st.Notes) != 0 {
		t.Fatalf("retuning retained old notes: %+v", st)
	}
}

func TestGapSilenceDoesNotAdvanceIntoNextNotes(t *testing.T) {
	p := New()
	p.Tune("a")
	p.Enqueue("a", seg(0, num(0)), samples(3*Rate, 100))
	p.EnqueueAnalyzed("a", seg(1, num(2)), samples(3*Rate, 100), nil, "helper offline")
	p.Read(make([]int16, 3*Rate))
	st := p.Status()
	if st.Position != -0.25 || st.AnalysisError != "helper offline" {
		t.Fatalf("gap must precede the next segment's note clock: %+v", st)
	}
	p.Read(make([]int16, pause))
	if st := p.Status(); st.Position != 0 {
		t.Fatalf("first sample should start at zero after gap: %+v", st)
	}
}
