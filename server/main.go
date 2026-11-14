// server receives audio segments from the piano nodes and serves them to the clients.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"rhapsody-rosarium/internal/api"
	"rhapsody-rosarium/internal/piano"
	"rhapsody-rosarium/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	config := flag.String("config", "pianos.json", "fixed list of pianos (id, name, position, token)")
	data := flag.String("data", "data", "where segments are stored")
	keep := flag.Int("keep", 30, "segments kept per piano")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "ffmpeg binary")
	flag.Parse()

	if _, err := exec.LookPath(*ffmpeg); err != nil {
		fatal("ffmpeg is needed to convert uploads", err)
	}
	pianos, err := piano.LoadConfig(*config)
	if err != nil {
		fatal("loading config", err)
	}
	st, err := store.NewStore(*data, *keep, *ffmpeg, pianos)
	if err != nil {
		fatal("opening store", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(st).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // uploads over a slow connection
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", *addr, "pianos", len(pianos), "keep", *keep)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fatal("serving", err)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
