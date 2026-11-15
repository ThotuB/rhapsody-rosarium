package main

import (
	"slices"
	"testing"
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
