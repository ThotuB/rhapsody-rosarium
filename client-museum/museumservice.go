package main

import (
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"rhapsody-rosarium/museum/internal/museum"
)

// MuseumService is what the museum's windows can call. Changes come back as "state" events.
type MuseumService struct {
	core     *museum.Core
	video    string // the projector's opening video, an absolute path (-video)
	introURL string // where the projector loads it from, see video.go
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

// RestartIntro plays the projector's opening video again from the start, from the museum window.
func (s *MuseumService) RestartIntro() {
	application.Get().Event.Emit("restart-intro")
}

func (s *MuseumService) Playback() museum.Playback {
	return s.core.Playback()
}
