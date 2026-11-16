package player

import (
	"math"
	"slices"
	"testing"
)

func sine(hz, amplitude float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = amplitude * math.Sin(2*math.Pi*hz*float64(i)/Rate)
	}
	return s
}

func TestSpectrumFindsTheNote(t *testing.T) {
	const bands = 64
	got := spectrum(sine(440, 0.5, fftSize), bands) // A4
	const want = 4                                  // 440 Hz, in bands of 93.75 Hz
	if loudest := slices.Index(got, slices.Max(got)); loudest != want {
		t.Errorf("loudest band %d, want %d (440 Hz): %.2f", loudest, want, got)
	}
	if got[want] < 0.9 {
		t.Errorf("a loud note should read near 1, got %.2f", got[want])
	}
	if got[bands-1] > 0.3 {
		t.Errorf("nothing near 6 kHz, got %.2f", got[bands-1])
	}
}

func TestSpectrumOfSilence(t *testing.T) {
	for i, v := range spectrum(make([]float64, fftSize), 32) {
		if v != 0 {
			t.Fatalf("band %d is %.2f, want 0", i, v)
		}
	}
}

func TestSpectrumFollowsWhatPlays(t *testing.T) {
	p := New()
	p.Tune("a")
	note := make([]int16, cushion)
	for i, v := range sine(440, 0.5, cushion) {
		note[i] = int16(v * math.MaxInt16)
	}
	p.Enqueue("a", seg(0, nil), note)
	if s := p.Spectrum(16); slices.Max(s) != 0 {
		t.Errorf("before anything plays: %.2f, want silence", s)
	}
	p.Read(make([]int16, fftSize))
	if s := p.Spectrum(16); s[1] < 0.9 { // 440 Hz is in the second band of 375 Hz
		t.Errorf("while the note plays: %.2f, want band 1 loud", s)
	}
}
