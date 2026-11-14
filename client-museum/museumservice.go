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
