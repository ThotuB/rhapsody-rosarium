//go:build linux && cgo && !gtk3

package windows

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

static GListModel *monitors(void) {
	return gdk_display_get_monitors(gdk_display_get_default());
}

static guint monitor_count(void) {
	return g_list_model_get_n_items(monitors());
}

// the monitor's connector, "eDP-1" or "HDMI-A-1"; a copy, freed by the caller
static char *monitor_connector(guint i) {
	GdkMonitor *monitor = g_list_model_get_item(monitors(), i);
	if (monitor == NULL) {
		return NULL;
	}
	char *connector = g_strdup(gdk_monitor_get_connector(monitor));
	g_object_unref(monitor);
	return connector;
}

static void fullscreen_on(void *window, guint i) {
	GdkMonitor *monitor = g_list_model_get_item(monitors(), i);
	if (monitor == NULL) {
		return;
	}
	gtk_window_fullscreen_on_monitor(GTK_WINDOW(window), monitor);
	g_object_unref(monitor);
}
*/
import "C"

import (
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// fullscreenOnMonitors fullscreens each window on its monitor through GTK: Wayland doesn't let apps
// move windows, so the SetScreen way doesn't work here. It's always handled, true.
func fullscreenOnMonitors(museum, projector *application.WebviewWindow) bool {
	application.InvokeAsync(func() {
		connectors := make([]string, C.monitor_count())
		for i := range connectors {
			if c := C.monitor_connector(C.guint(i)); c != nil {
				connectors[i] = C.GoString(c)
				C.g_free(C.gpointer(c))
			}
		}
		laptop, other, ok := laptopAndProjector(connectors)
		if !ok {
			slog.Info("one screen, the windows stay windows", "screens", connectors)
			return
		}
		slog.Info("fullscreen windows", "screens", connectors, "museum", connectors[laptop], "projector", connectors[other])
		C.fullscreen_on(museum.NativeWindow(), C.guint(laptop))
		C.fullscreen_on(projector.NativeWindow(), C.guint(other))
	})
	return true
}
