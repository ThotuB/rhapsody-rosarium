// The museum controller: plays the pianos on the installation's speakers and shows them in its
// windows, an interface on the installation's screen and visuals on the projector.
package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"rhapsody-rosarium/museum/internal/audio"
	"rhapsody-rosarium/museum/internal/bulbs"
	"rhapsody-rosarium/museum/internal/config"
	"rhapsody-rosarium/museum/internal/museum"
	"rhapsody-rosarium/museum/internal/player"
	"rhapsody-rosarium/museum/internal/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// bands in the "spectrum" events, for the rose in the museum window
const spectrumBands = 64

// the app's id on Linux; build/linux/install-desktop.sh names its desktop entry after it
const appID = "ro.rhapsodyrosarium.Museum"

func init() {
	// typed events, the binding generator picks these up for the frontend
	application.RegisterEvent[museum.View]("state")
	application.RegisterEvent[museum.Playback]("playback")
	application.RegisterEvent[float64]("level")      // loudness of what's playing, 0 to 1, ~20 times a second
	application.RegisterEvent[[]float64]("spectrum") // of what's playing, spectrumBands from 0 to 6 kHz, each 0 to 1, ~20 times a second
}

func main() {
	configPath := flag.String("config", filepath.Join("config", "museum-bulbs.json"), "the installation's settings: server, lights board, which piano each strip follows")
	server := flag.String("server", "", "the rhapsody rosarium server (overrides the config; default "+config.Defaults.Server+")")
	video := flag.String("video", filepath.Join("media", "intro.mp4"), "the projector's opening video (media/ is in git lfs)")
	tune := flag.String("tune", "", "piano to tune in to at startup")
	sink := flag.String("sink", "device", `where audio goes: "device" (the sound card) or "null" (nowhere, for testing)`)
	headless := flag.Bool("headless", false, "run without windows, logging what plays")
	runFor := flag.Duration("for", 0, "with -headless, stop after this long (0 runs until Ctrl+C)")
	lightsPort := flag.String("lights", "",
		`the board driving the led strips (Magma Splash, Carbon V3 or Arduino): a port like /dev/ttyACM0, "auto" (the first one plugged in), "log" (only log what it'd send) or "off" (overrides the config; default auto)`)
	strips := flag.String("strips", "",
		`comma separated piano ids the led strips follow, in strip order; an empty slot ("piano-2,,piano-4") leaves that strip off (overrides the config; default: the server's first pianos)`)
	flag.Parse()

	asked := false
	flag.Visit(func(f *flag.Flag) { asked = asked || f.Name == "config" })
	cfg, err := config.Load(*configPath, asked)
	if err != nil {
		log.Fatal(err)
	}
	cfg.OverrideWithFlags(*server, *lightsPort, *strips)

	p := player.New()
	out, err := openOutput(*sink, p.Read)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	core := museum.New(cfg.Server, p)
	if *tune != "" {
		core.Tune(*tune)
	}

	ctx, stopLights := context.WithCancel(context.Background())
	lightsOff := bulbs.Start(ctx, cfg.Lights, cfg.Strips, core, func() float64 { return p.Status().Level })
	defer func() {
		stopLights()
		select { // a moment to switch the strips off
		case <-lightsOff:
		case <-time.After(2 * time.Second):
		}
	}()
	if *headless {
		runHeadless(core, *runFor)
		return
	}
	runWindows(core, p, *video)
}

func openOutput(sink string, src audio.Source) (audio.Output, error) {
	if sink == "null" {
		return audio.OpenNull(src), nil
	}
	return audio.OpenDevice(src)
}

// runHeadless plays without windows and logs the player's state every few seconds.
func runHeadless(core *museum.Core, runFor time.Duration) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if runFor > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, runFor)
		defer cancel()
	}
	done := make(chan struct{})
	go func() {
		core.Run(ctx)
		close(done)
	}()

	report := func() {
		v := core.View()
		seq := "-"
		if v.Segment != nil && v.Segment.Seq != nil {
			seq = strconv.FormatInt(*v.Segment.Seq, 10)
		}
		slog.Info("museum", "tuned", v.Tuned, "status", v.Status, "buffered", math.Round(v.Buffered*10)/10,
			"behind", math.Round(v.Behind*10)/10, "seq", seq, "underruns", v.Underruns, "server", v.ServerError)
	}
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			report()
			<-done // lets it tell the server the museum stopped listening
			return
		case <-tick.C:
			report()
		}
	}
}

// runWindows opens the interface and projector windows and pushes state and loudness to them.
func runWindows(core *museum.Core, p *player.Player, video string) {
	// resolved once, against the folder the app started from, and logged, so a missing video says where it looked
	if abs, err := filepath.Abs(video); err == nil {
		video = abs
	}
	introURL := windows.ServeIntroVideo(video)
	if info, err := os.Stat(video); err != nil {
		slog.Warn("projector video not found, the projector will offer the map instead", "path", video)
	} else {
		slog.Info("projector video", "path", video, "MB", info.Size()>>20, "url", introURL)
	}

	app := application.New(application.Options{
		Name:        "Rhapsody Rosarium museum",
		Description: "The museum installation of Rhapsody Rosarium",
		Services: []application.Service{
			// the projector opens on the video, waiting to be started
			application.NewService(&MuseumService{core: core, video: video, introURL: introURL, intro: IntroState{Phase: "intro"}}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == windows.IntroPath { // on Linux it's served from 127.0.0.1 instead, see windows.ServeIntroVideo
						windows.IntroHandler(video).ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// the windows' Wayland app id: GNOME shows the icon of the .desktop file named after it, see
		// build/linux/install-desktop.sh
		Linux: application.LinuxOptions{ApplicationID: appID},
	})

	museumWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Rhapsody Rosarium · museum",
		Width:            1200,
		Height:           760,
		BackgroundColour: application.NewRGB(20, 18, 22),
		URL:              "/",
	})
	projector := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Rhapsody Rosarium · projector",
		Width:            960,
		Height:           540,
		BackgroundColour: application.NewRGB(0, 0, 0),
		URL:              "/?view=projector",
	})
	// with a second display, the museum window fullscreen on the laptop and the projector on the other
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		windows.FullscreenOnScreens(app, museumWindow, projector)
	})

	core.OnChange(func() { app.Event.Emit("state", core.View()) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		core.Run(ctx)
		close(done)
	}()
	go func() {
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				app.Event.Emit("level", p.Status().Level)
				app.Event.Emit("spectrum", p.Spectrum(spectrumBands))
				app.Event.Emit("playback", core.Playback())
				if i%10 == 0 { // playing / waiting changes on the audio side, between polls
					app.Event.Emit("state", core.View())
				}
			}
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	cancel()
	<-done // lets it tell the server the museum stopped listening
}
