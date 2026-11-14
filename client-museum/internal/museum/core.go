// Package museum is the installation's core: it keeps the server's state, decides what to play
// and feeds the player. It runs the same with or without windows.
package museum

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"rhapsody-rosarium/museum/internal/link"
	"rhapsody-rosarium/museum/internal/player"
)

const pollEvery = time.Second // new segments are picked up within this, part of the delay behind the piano

// View is everything the museum's windows show, sent to them as the "state" event.
type View struct {
	Now         time.Time     `json:"now"` // the server's clock at the last poll
	Pianos      []link.Piano  `json:"pianos"`
	Tuned       string        `json:"tuned"`       // the tuned piano's id, "" when off
	Status      string        `json:"status"`      // "off", "playing" or "waiting"
	Segment     *link.Segment `json:"segment"`     // the segment playing now
	Buffered    float64       `json:"buffered"`    // seconds of audio lined up
	Behind      float64       `json:"behind"`      // seconds since the piano played what's playing now
	Underruns   int           `json:"underruns"`   // times playback ran dry
	ServerError string        `json:"serverError"` // why the last poll failed, "" if it worked
}

type Core struct {
	link   *link.Client
	player *player.Player
	kick   chan struct{} // poll right away, e.g. after tuning in

	mu       sync.Mutex
	state    link.State
	err      string
	onChange func()
}

func New(server string, p *player.Player) *Core {
	return &Core{link: link.New(server), player: p, kick: make(chan struct{}, 1)}
}

// OnChange sets a function called whenever the view may have changed.
func (c *Core) OnChange(f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = f
}

func (c *Core) Tune(piano string) {
	c.player.Tune(piano)
	select {
	case c.kick <- struct{}{}:
	default:
	}
	c.changed()
}

func (c *Core) Stop() {
	c.Tune("")
}

// Run polls the server and feeds the player until ctx is done.
func (c *Core) Run(ctx context.Context) {
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	for {
		c.step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-c.kick:
		}
	}
}

func (c *Core) step(ctx context.Context) {
	state, err := c.link.State(ctx)
	c.mu.Lock()
	if err != nil {
		c.err = err.Error()
	} else {
		c.state, c.err = state, ""
	}
	c.mu.Unlock()
	if err == nil {
		c.feed(ctx, state)
	}
	c.changed()
}

// feed downloads what the tuned piano has that the player still needs, in order.
func (c *Core) feed(ctx context.Context, state link.State) {
	tuned := c.player.Tuned()
	for _, p := range state.Pianos {
		if p.ID != tuned {
			continue
		}
		for _, seg := range c.player.Want(tuned, player.Playable(p, state.Now)) {
			wav, err := c.link.Download(ctx, seg)
			if err != nil {
				slog.Warn("downloading segment", "url", seg.URL, "err", err) // e.g. already pruned, skipped
				continue
			}
			samples, err := player.DecodeWAV(wav)
			if err != nil {
				slog.Warn("decoding segment", "url", seg.URL, "err", err)
				continue
			}
			c.player.Enqueue(tuned, seg, samples)
		}
	}
}

func (c *Core) View() View {
	st := c.player.Status()
	c.mu.Lock()
	defer c.mu.Unlock()
	v := View{
		Now:         c.state.Now,
		Pianos:      c.state.Pianos,
		Tuned:       st.Piano,
		Segment:     st.Segment,
		Buffered:    st.Buffered.Seconds(),
		Behind:      st.Behind.Seconds(),
		Underruns:   st.Underruns,
		ServerError: c.err,
	}
	switch {
	case st.Piano == "":
		v.Status = "off"
	case st.Buffered > 0 && !st.Holding:
		v.Status = "playing"
	default:
		v.Status = "waiting"
	}
	if v.Pianos == nil {
		v.Pianos = []link.Piano{}
	}
	return v
}

func (c *Core) changed() {
	c.mu.Lock()
	f := c.onChange
	c.mu.Unlock()
	if f != nil {
		f()
	}
}
