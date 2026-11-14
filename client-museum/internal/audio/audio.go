// Package audio connects a source of samples to the sound card, or to a silent stand-in for testing.
package audio

import (
	"sync"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
)

const rate = 48000

// Source fills out with the next mono 48 kHz samples, silence when there's nothing to play.
type Source func(out []int16)

type Output interface {
	Close()
}

type device struct {
	ctx *malgo.AllocatedContext
	dev *malgo.Device
}

// OpenDevice plays the source on the default sound card. miniaudio loads the system's sound
// libraries at runtime (pipewire, pulse, alsa), so no development packages are needed.
func OpenDevice(src Source) (Output, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatS16
	cfg.Playback.Channels = 1
	cfg.SampleRate = rate
	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(out, _ []byte, frames uint32) {
			if frames == 0 || len(out) < int(frames)*2 {
				return
			}
			src(unsafe.Slice((*int16)(unsafe.Pointer(&out[0])), frames))
		},
	})
	if err != nil {
		ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	if err := dev.Start(); err != nil {
		dev.Uninit()
		ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	return &device{ctx: ctx, dev: dev}, nil
}

func (d *device) Close() {
	d.dev.Uninit()
	d.ctx.Uninit()
	d.ctx.Free()
}

type null struct {
	stop chan struct{}
	once sync.Once
}

// OpenNull reads the source in real time like a sound card would, but plays nothing.
// For running the museum headless without making noise, e.g. to check for underruns.
func OpenNull(src Source) Output {
	n := &null{stop: make(chan struct{})}
	go func() {
		buf := make([]int16, rate)
		start := time.Now()
		var read int
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-n.stop:
				return
			case <-tick.C:
				due := int(time.Since(start).Seconds()*rate) - read
				for due > 0 {
					k := min(due, len(buf))
					src(buf[:k])
					read += k
					due -= k
				}
			}
		}
	}()
	return n
}

func (n *null) Close() {
	n.once.Do(func() { close(n.stop) })
}
