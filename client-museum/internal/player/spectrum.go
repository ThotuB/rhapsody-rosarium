package player

import (
	"math"
	"math/cmplx"
)

// The spectrum is measured like a browser's AnalyserNode with its defaults, so the museum's rose
// looks like the web app's, which draws from one.
const (
	fftSize      = 2048   // samples measured, the last ~43 ms played
	spectrumTop  = 6000.0 // Hz, the highest band: a piano barely reaches above
	minDecibels  = -100.0 // what reads as 0
	maxDecibels  = -30.0  // what reads as 1
	spectrumBins = fftSize * spectrumTop / Rate
)

// Spectrum is how loud the last samples played are across frequencies, in bands from 0 to 6 kHz,
// each 0 to 1 on a decibel scale; all 0 while nothing plays.
func (p *Player) Spectrum(bands int) []float64 {
	samples := make([]float64, fftSize)
	p.mu.Lock()
	if !p.holding { // holding is silent, whatever is lined up
		start := p.pos - fftSize
		for i := range samples {
			if j := start + i; j >= 0 && j < len(p.buf) {
				samples[i] = float64(p.buf[j]) / math.MaxInt16
			}
		}
	}
	p.mu.Unlock()
	return spectrum(samples, bands)
}

// spectrum turns fftSize samples into bands from 0 to spectrumTop, each the loudest frequency in it.
func spectrum(samples []float64, bands int) []float64 {
	x := make([]complex128, len(samples))
	for i, s := range samples { // Blackman window, as AnalyserNode
		a := 2 * math.Pi * float64(i) / float64(len(samples))
		x[i] = complex(s*(0.42-0.5*math.Cos(a)+0.08*math.Cos(2*a)), 0)
	}
	fft(x)

	out := make([]float64, bands)
	for b := range out {
		from := b * spectrumBins / bands
		to := max(from+1, (b+1)*spectrumBins/bands)
		for k := from; k < to; k++ {
			db := 20 * math.Log10(cmplx.Abs(x[k])/float64(len(x))+1e-12)
			out[b] = max(out[b], min(1, max(0, (db-minDecibels)/(maxDecibels-minDecibels))))
		}
	}
	return out
}

// fft is an in-place radix-2 FFT; len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ { // bit-reversed order
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := range size / 2 {
				a, b := x[start+k], x[start+k+size/2]*w
				x[start+k], x[start+k+size/2] = a+b, a-b
				w *= step
			}
		}
	}
}
