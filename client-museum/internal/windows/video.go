package windows

import (
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"time"
)

// The projector's opening video. The app's own files reach its windows through Wails' custom
// scheme (wails://) on Linux, and WebKitGTK's video player can't stream from that: the video
// fails with "source not supported" (media error 4) although the file and its decoder are fine.
// So on Linux the video comes from a small web server on 127.0.0.1 instead, where the player
// gets ordinary range requests; elsewhere (WebView2 on Windows) it stays on the app's own path.

const IntroPath = "/projector-intro.mp4"

// ServeIntroVideo returns the URL the projector loads the video from, starting the loopback
// server first on Linux. It falls back to the app's own path if that server can't start.
func ServeIntroVideo(video string) string {
	if runtime.GOOS != "linux" {
		return IntroPath
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0") // any free port, reachable only from this machine
	if err != nil {
		slog.Warn("projector video: no loopback server, trying the app's own path", "err", err)
		return IntroPath
	}
	srv := &http.Server{Handler: IntroHandler(video), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	return "http://" + ln.Addr().String() + IntroPath
}

// IntroHandler serves the video file at IntroPath, with range requests for seeking and
// streaming; it's read from disk, not embedded in the build.
func IntroHandler(video string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != IntroPath {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, video)
	})
}
