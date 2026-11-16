// Package lights drives the museum's led strips through the board running lights-board/lights-board.ino
// (a GroundStudio Magma Splash, a Carbon V3 or an Arduino), over USB serial. It keeps what every strip
// should show and sends it again whenever the board (re)connects, so unplugging it or a reset never
// leaves the strips wrong. It never blocks its callers: a missing board only means the strips stay dark.
package lights

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Strips is how many strips the firmware drives.
const Strips = 6

var retryEvery = 2 * time.Second // a var so tests can shorten it

const (
	baud = 115200
	// a board is asked with "?" as soon as the port is open, which a Carbon V3 answers right away.
	// opening the port resets an Uno / Nano instead, which prints READY once it has started (and
	// misses that first "?"), so it's asked again once this has passed
	askAgain  = 3 * time.Second
	readyWait = 5 * time.Second
)

// Strip is what one strip shows.
type Strip struct {
	Level  uint8         // brightness 0-255, at the brightest point of a pulse
	Fade   time.Duration // how long to fade to a new level
	Pulses float64       // pulses per second between off and Level, 0 = steady
}

// off is every strip fading out, sent when the museum controller quits.
var off = func() (s [Strips]Strip) {
	for i := range s {
		s[i].Fade = 500 * time.Millisecond
	}
	return s
}()

// Port is the serial connection to the board.
type Port = io.ReadWriteCloser

type Lights struct {
	open func() (Port, string, error) // the port and its name

	mu   sync.Mutex
	want [Strips]Strip
	wake chan struct{}
}

// New drives the strips through the board on port: a device path, "auto" for the first board
// found (/dev/ttyUSB* or /dev/ttyACM*), or "log" to only log the commands, without hardware.
func New(port string) *Lights {
	switch port {
	case "log":
		return newLights(openLog)
	case "auto":
		return newLights(openAuto)
	default:
		return newLights(func() (Port, string, error) { return openSerial(port) })
	}
}

func newLights(open func() (Port, string, error)) *Lights {
	return &Lights{open: open, wake: make(chan struct{}, 1)}
}

// Set changes what strip i shows. Only changes reach the board.
func (l *Lights) Set(i int, s Strip) {
	if i < 0 || i >= Strips {
		return
	}
	l.mu.Lock()
	changed := l.want[i] != s
	l.want[i] = s
	l.mu.Unlock()
	if changed {
		select {
		case l.wake <- struct{}{}:
		default:
		}
	}
}

// Run connects to the board and keeps the strips up to date until ctx is done, reconnecting
// whenever the board goes away.
func (l *Lights) Run(ctx context.Context) {
	failing := ""
	for ctx.Err() == nil {
		err := l.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if msg := err.Error(); msg != failing { // log each new problem once, not every retry
			slog.Warn("lights", "err", err)
			failing = msg
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryEvery):
		}
	}
}

// session is one connection to the board, from opening the port until it fails.
func (l *Lights) session(ctx context.Context) error {
	port, name, err := l.open()
	if err != nil {
		return err
	}
	defer port.Close()

	lines := make(chan string)
	dead := make(chan error, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		scan := bufio.NewScanner(port)
		for scan.Scan() {
			select {
			case lines <- strings.TrimSpace(scan.Text()):
			case <-done:
				return
			}
		}
		dead <- orEOF(scan.Err())
	}()

	if err := waitReady(ctx, port, lines, dead); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	slog.Info("lights connected", "port", name)

	var sent [Strips]Strip
	fresh := true // nothing sent yet on this connection: send every strip
	for {
		l.mu.Lock()
		want := l.want
		l.mu.Unlock()
		if err := send(port, &sent, want, fresh); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fresh = false

		select {
		case <-ctx.Done(): // shutting down: the strips go dark rather than keep the last state
			send(port, &sent, off, false)
			return ctx.Err()
		case err := <-dead:
			return fmt.Errorf("%s: %w", name, err)
		case line := <-lines:
			if strings.HasPrefix(line, "READY") { // the board restarted, it knows nothing
				fresh = true
			}
		case <-l.wake:
		}
	}
}

func waitReady(ctx context.Context, port Port, lines <-chan string, dead <-chan error) error {
	if _, err := io.WriteString(port, "?\n"); err != nil {
		return err
	}
	ask := time.After(askAgain)
	give := time.After(readyWait)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-dead:
			return err
		case line := <-lines:
			if strings.HasPrefix(line, "READY") {
				return nil
			}
		case <-ask:
			if _, err := io.WriteString(port, "?\n"); err != nil {
				return err
			}
		case <-give:
			return errors.New("no READY from the board, is lights-board.ino on it?")
		}
	}
}

// send writes the commands that take the board from sent to want, and updates sent.
func send(port Port, sent *[Strips]Strip, want [Strips]Strip, all bool) error {
	var b strings.Builder
	for i, w := range want {
		s := sent[i]
		if all || w.Level != s.Level {
			fmt.Fprintf(&b, "L %d %d %d\n", i, w.Level, w.Fade.Milliseconds())
		}
		if all || w.Pulses != s.Pulses {
			fmt.Fprintf(&b, "P %d %.3g\n", i, w.Pulses)
		}
		sent[i] = w
	}
	if b.Len() == 0 {
		return nil
	}
	_, err := io.WriteString(port, b.String())
	return err
}

// orEOF is why reading stopped: a scanner reports the port closing as no error at all.
func orEOF(err error) error {
	if err == nil {
		return io.EOF
	}
	return err
}

func openSerial(path string) (Port, string, error) {
	port, err := serial.Open(path, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, path, fmt.Errorf("%s: %w", path, err)
	}
	closeOnExec(path)
	return port, path, nil
}

func openAuto() (Port, string, error) {
	// the Carbon V3 (and Nano clones) have a CH340 usb chip and show up as ttyUSB, an original Uno
	// / Nano and boards with usb on the chip itself (the Magma Splash) as ttyACM
	for _, pattern := range []string{"/dev/ttyUSB*", "/dev/ttyACM*"} {
		if found, _ := filepath.Glob(pattern); len(found) > 0 {
			return openSerial(found[0])
		}
	}
	return nil, "", errors.New("no board plugged in (no /dev/ttyUSB* or /dev/ttyACM*)")
}

// openLog stands in for a board: it logs every command and answers like the firmware would.
func openLog() (Port, string, error) {
	r, w := io.Pipe()
	go io.WriteString(w, fmt.Sprintf("READY %d\n", Strips))
	return &logPort{r: r, w: w}, "log", nil
}

type logPort struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (p *logPort) Read(b []byte) (int, error) { return p.r.Read(b) }

func (p *logPort) Write(b []byte) (int, error) {
	for line := range strings.Lines(string(b)) {
		if line = strings.TrimSpace(line); line != "?" { // the readiness check isn't interesting
			slog.Info("lights", "send", line)
		}
	}
	return len(b), nil
}

func (p *logPort) Close() error {
	p.w.Close()
	return p.r.Close()
}
