//go:build !linux || !cgo || gtk3

package windows

import "github.com/wailsapp/wails/v3/pkg/application"

// fullscreenOnMonitors is only needed on Linux (see screens_linux.go); elsewhere SetScreen works.
func fullscreenOnMonitors(museum, projector *application.WebviewWindow) bool {
	return false
}
