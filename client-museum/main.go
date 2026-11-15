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
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"rhapsody-rosarium/museum/internal/audio"
	"rhapsody-rosarium/museum/internal/museum"
	"rhapsody-rosarium/museum/internal/player"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// typed events, the binding generator picks these up for the frontend
	application.RegisterEvent[museum.View]("state")
	application.RegisterEvent[float64]("level") // loudness of what's playing, 0 to 1, ~20 times a second
}

func main() {
	configPath := flag.String("config", "museum-bulbs.json", "the installation's settings: server, lights board, which piano each strip follows")
	server := flag.String("server", "", "the rhapsody rosarium server (overrides the config; default "+defaults.Server+")")
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
	cfg, err := loadConfig(*configPath, asked)
	if err != nil {
		log.Fatal(err)
	}
	cfg.overrideWithFlags(*server, *lightsPort, *strips)

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
	lightsOff := startLights(ctx, cfg.Lights, cfg.Strips, core, func() float64 { return p.Status().Level })
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
	runWindows(core, p)
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
func runWindows(core *museum.Core, p *player.Player) {
	app := application.New(application.Options{
		Name:        "Rhapsody Rosarium museum",
		Description: "The museum installation of Rhapsody Rosarium",
		Services: []application.Service{
			application.NewService(&MuseumService{core: core}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
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
	// fullscreen on a second display when there is one; with a single screen it stays a normal
	// window, so it doesn't cover the only screen
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		screens := app.Screen.GetAll()
		if len(screens) < 2 {
			return
		}
		// the non-primary one, if exactly one is marked primary; on wayland often none is, then the second
		target := screens[1]
		primaries := 0
		for _, s := range screens {
			if s.IsPrimary {
				primaries++
			}
		}
		if primaries == 1 {
			for _, s := range screens {
				if !s.IsPrimary {
					target = s
					break
				}
			}
		}
		projector.SetScreen(target)
		projector.Fullscreen()
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
