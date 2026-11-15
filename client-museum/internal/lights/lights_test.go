package lights

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"
)

// board is a fake board on the other end of an in-memory pipe, recording the strip commands it's
// sent, and the "?" queries apart from them.
type board struct {
	conn  net.Conn
	lines chan string
	asked chan struct{}
}

func newBoard() (*board, net.Conn) {
	ours, theirs := net.Pipe()
	b := &board{conn: ours, lines: make(chan string, 100), asked: make(chan struct{}, 10)}
	go func() {
		scan := bufio.NewScanner(ours)
		for scan.Scan() {
			if scan.Text() == "?" {
				b.asked <- struct{}{}
				continue
			}
			b.lines <- scan.Text()
		}
		close(b.lines)
	}()
	return b, theirs
}

func (b *board) ready() { fmt.Fprintf(b.conn, "READY %d\n", Strips) }

// next returns the next n lines the board got.
func (b *board) next(t *testing.T, n int) []string {
	t.Helper()
	var got []string
	for len(got) < n {
		select {
		case line, ok := <-b.lines:
			if !ok {
				t.Fatalf("wanted %d lines, the port closed after %v", n, got)
			}
			got = append(got, line)
		case <-time.After(time.Second):
			t.Fatalf("wanted %d lines, got %v", n, got)
		}
	}
	return got
}

func (b *board) quiet(t *testing.T) {
	t.Helper()
	select {
	case line := <-b.lines:
		t.Fatalf("expected nothing, got %q", line)
	case <-time.After(200 * time.Millisecond):
	}
}

// connected starts Lights with each open handing out the next fake board's end.
func connected(t *testing.T) (*Lights, chan net.Conn) {
	t.Helper()
	ports := make(chan net.Conn, 4)
	l := newLights(func() (Port, string, error) {
		select {
		case p := <-ports:
			return p, "fake", nil
		case <-time.After(time.Second):
			return nil, "", fmt.Errorf("no fake board")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go l.Run(ctx)
	return l, ports
}

// all is what a fresh board is sent: every strip, as set
func all(set map[int]Strip) []string {
	var lines []string
	for i := range Strips {
		s := set[i]
		lines = append(lines, fmt.Sprintf("L %d %d %d", i, s.Level, s.Fade.Milliseconds()), fmt.Sprintf("P %d %.3g", i, s.Pulses))
	}
	return lines
}

func TestWaitsForReadyThenSendsEverything(t *testing.T) {
	l, ports := connected(t)
	l.Set(1, Strip{Level: 200, Fade: 300 * time.Millisecond, Pulses: 0.5})
	b, port := newBoard()
	ports <- port

	b.quiet(t) // the board is still starting up
	b.ready()
	want := all(map[int]Strip{1: {Level: 200, Fade: 300 * time.Millisecond, Pulses: 0.5}})
	if got := b.next(t, len(want)); !slices.Equal(got, want) {
		t.Errorf("after READY:\n got %q\nwant %q", got, want)
	}
}

// a Carbon V3 doesn't restart when the port opens, so it never prints READY by itself: it's asked
func TestAsksBoardsThatDontRestart(t *testing.T) {
	l, ports := connected(t)
	l.Set(0, Strip{Level: 10})
	b, port := newBoard()
	ports <- port

	select {
	case <-b.asked:
	case <-time.After(time.Second):
		t.Fatal("the board wasn't asked whether it's ready")
	}
	b.ready() // the firmware's answer to "?"
	want := all(map[int]Strip{0: {Level: 10}})
	if got := b.next(t, len(want)); !slices.Equal(got, want) {
		t.Errorf("after answering:\n got %q\nwant %q", got, want)
	}
}

func TestSendsOnlyChanges(t *testing.T) {
	l, ports := connected(t)
	b, port := newBoard()
	ports <- port
	b.ready()
	b.next(t, 2*Strips)

	l.Set(2, Strip{Level: 128, Fade: 500 * time.Millisecond})
	if got := b.next(t, 1); got[0] != "L 2 128 500" {
		t.Errorf("new level: %q", got)
	}
	l.Set(2, Strip{Level: 128, Fade: 500 * time.Millisecond})
	b.quiet(t)
	l.Set(2, Strip{Level: 128, Fade: 500 * time.Millisecond, Pulses: 0.75})
	if got := b.next(t, 1); got[0] != "P 2 0.75" {
		t.Errorf("new pulse speed: %q", got)
	}
	l.Set(Strips, Strip{Level: 255}) // no such strip
	b.quiet(t)
}

func TestResendsWhenTheBoardRestarts(t *testing.T) {
	l, ports := connected(t)
	b, port := newBoard()
	ports <- port
	b.ready()
	b.next(t, 2*Strips)
	l.Set(0, Strip{Level: 255})
	b.next(t, 1)

	b.ready() // reset, e.g. a power glitch
	want := all(map[int]Strip{0: {Level: 255}})
	if got := b.next(t, len(want)); !slices.Equal(got, want) {
		t.Errorf("after a restart:\n got %q\nwant %q", got, want)
	}
}

func TestSwitchesOffWhenStopped(t *testing.T) {
	ports := make(chan net.Conn, 1)
	l := newLights(func() (Port, string, error) { return <-ports, "fake", nil })
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(stopped)
	}()
	b, port := newBoard()
	ports <- port
	b.ready()
	b.next(t, 2*Strips)
	l.Set(3, Strip{Level: 255, Pulses: 2})
	b.next(t, 2)

	cancel()
	if got := b.next(t, 2); !slices.Equal(got, []string{"L 3 0 500", "P 3 0"}) {
		t.Errorf("on stopping, the lit strip should fade out: %q", got)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Run didn't return")
	}
}

func TestReconnects(t *testing.T) {
	retryEvery = 10 * time.Millisecond
	t.Cleanup(func() { retryEvery = 2 * time.Second })

	l, ports := connected(t)
	first, port := newBoard()
	ports <- port
	first.ready()
	first.next(t, 2*Strips)
	l.Set(4, Strip{Level: 90, Pulses: 1})
	first.next(t, 2)

	first.conn.Close() // unplugged
	second, port := newBoard()
	ports <- port
	second.ready()
	want := all(map[int]Strip{4: {Level: 90, Pulses: 1}})
	if got := second.next(t, len(want)); !slices.Equal(got, want) {
		t.Errorf("after plugging back in:\n got %q\nwant %q", got, want)
	}
}
