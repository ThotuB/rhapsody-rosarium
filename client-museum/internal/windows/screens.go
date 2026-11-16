// Package windows is what the museum's two windows need around them: going fullscreen on the right
// screens, and serving the projector's opening video.
package windows

import (
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// FullscreenOnScreens puts the museum window fullscreen on the laptop's own screen and the projector
// window fullscreen on the other one. With a single screen both stay normal windows, so neither
// covers the only screen. Call it once the app has started.
func FullscreenOnScreens(app *application.App, museum, projector *application.WebviewWindow) {
	if fullscreenOnMonitors(museum, projector) { // Linux, see screens_linux.go
		return
	}
	screens := app.Screen.GetAll()
	if len(screens) < 2 {
		return
	}
	names := make([]string, len(screens))
	for i, s := range screens {
		names[i] = s.Name
	}
	laptop, other := 0, 1
	// the primary one for the museum, if exactly one is marked primary
	primaries := 0
	for _, s := range screens {
		if s.IsPrimary {
			primaries++
		}
	}
	if primaries == 1 {
		for i, s := range screens {
			if s.IsPrimary {
				laptop = i
			}
		}
		other = otherThan(laptop)
	}
	slog.Info("fullscreen windows", "screens", names, "museum", names[laptop], "projector", names[other])
	museum.SetScreen(screens[laptop])
	museum.Fullscreen()
	projector.SetScreen(screens[other])
	projector.Fullscreen()
}

// laptopAndProjector picks, from the monitors' connector names ("eDP-1", "HDMI-A-1"), the laptop's own
// panel for the museum window (the first one, if none looks built in) and another for the projector.
// ok is false with fewer than two monitors.
func laptopAndProjector(connectors []string) (laptop, projector int, ok bool) {
	if len(connectors) < 2 {
		return 0, 0, false
	}
	for i, c := range connectors {
		if builtIn(c) {
			laptop = i
			break
		}
	}
	return laptop, otherThan(laptop), true
}

// builtIn says whether a connector is a laptop's own panel.
func builtIn(connector string) bool {
	for _, prefix := range []string{"eDP", "LVDS", "DSI"} {
		if strings.HasPrefix(connector, prefix) {
			return true
		}
	}
	return false
}

// otherThan is the first screen that isn't i.
func otherThan(i int) int {
	if i == 0 {
		return 1
	}
	return 0
}
