package main

import "rhapsody-rosarium/museum/internal/museum"

// MuseumService is what the museum's windows can call. Changes come back as "state" events.
type MuseumService struct {
	core *museum.Core
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

func (s *MuseumService) Playback() museum.Playback {
	return s.core.Playback()
}
