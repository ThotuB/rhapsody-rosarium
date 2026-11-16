package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"rhapsody-rosarium/museum/internal/museum"
)

// MuseumService is what the museum's windows can call. Changes come back as "state" events.
type MuseumService struct {
	core     *museum.Core
	video    string // the projector's opening video, an absolute path (-video)
	introURL string // where the projector loads it from, see internal/windows/video.go

	mu    sync.Mutex
	intro IntroState // as the projector last reported it
}

// IntroVideo is where the projector's opening video is read from, and whether a file is there.
type IntroVideo struct {
	URL   string `json:"url"`   // what the projector's <video> loads
	Path  string `json:"path"`  // the file, absolute
	Found bool   `json:"found"` // a file is there
	Size  int64  `json:"size"`  // in bytes, 0 if not found
}

// IntroVideo says where the opening video comes from, so the projector can load it, and say where it
// looked when it can't play it.
func (s *MuseumService) IntroVideo() IntroVideo {
	v := IntroVideo{URL: s.introURL, Path: s.video}
	if info, err := os.Stat(s.video); err == nil && info.Mode().IsRegular() {
		v.Found, v.Size = true, info.Size()
	}
	return v
}

// State returns the current view, for a window that just opened.
func (s *MuseumService) State() museum.View {
	return s.core.View()
}

// Tune plays a piano on the installation's speakers.
func (s *MuseumService) Tune(piano string) {
	s.core.Tune(piano)
}

func (s *MuseumService) Stop() {
	s.core.Stop()
}

// SetLED sets the colour of the light at a piano, "#rrggbb". Its node shows it within a heartbeat.
func (s *MuseumService) SetLED(piano, colour string) error {
	return s.core.SetLED(piano, colour)
}

// IntroState is what the projector's opening video is doing, for the museum window's controls.
type IntroState struct {
	Phase   string `json:"phase"`   // "intro" (the video, playing or waiting to start) or "map"
	Playing bool   `json:"playing"` // false while it waits to be started
	Muted   bool   `json:"muted"`
}

// IntroAction is what the museum window's controls ask the projector's opening video to do.
type IntroAction string

const (
	IntroStart  IntroAction = "start" // play it from the start, also from the map
	IntroMute   IntroAction = "mute"
	IntroUnmute IntroAction = "unmute"
	IntroSkip   IntroAction = "skip" // to the map
)

// ControlIntro passes one of the museum window's intro controls to the projector, as an
// "intro-action" event: the projector itself has no controls.
func (s *MuseumService) ControlIntro(action IntroAction) error {
	switch action {
	case IntroStart, IntroMute, IntroUnmute, IntroSkip:
		application.Get().Event.Emit("intro-action", action)
		return nil
	}
	return fmt.Errorf("unknown intro action %q", action)
}

// ReportIntro is the projector saying what its opening video does, passed on to the museum window
// as an "intro" event.
func (s *MuseumService) ReportIntro(state IntroState) {
	s.mu.Lock()
	s.intro = state
	s.mu.Unlock()
	application.Get().Event.Emit("intro", state)
}

// Intro is what the projector's opening video does, for a window that just opened.
func (s *MuseumService) Intro() IntroState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intro
}

func (s *MuseumService) Playback() museum.Playback {
	return s.core.Playback()
}
