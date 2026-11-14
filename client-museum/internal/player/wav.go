package player

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// DecodeWAV returns the samples of a wav file as the server stores them: 48 kHz mono 16-bit PCM.
// It walks the RIFF chunks to find "fmt " and "data", since ffmpeg adds a LIST chunk before the audio.
func DecodeWAV(b []byte) ([]int16, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("not a wav file")
	}
	var formatOK bool
	for i := 12; i+8 <= len(b); {
		id := string(b[i : i+4])
		size := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		body := b[i+8:]
		if size > len(body) {
			size = len(body) // a truncated last chunk, or a streaming writer's placeholder size
		}
		body = body[:size]
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, errors.New("short fmt chunk")
			}
			format := binary.LittleEndian.Uint16(body[0:2])
			channels := binary.LittleEndian.Uint16(body[2:4])
			rate := binary.LittleEndian.Uint32(body[4:8])
			bits := binary.LittleEndian.Uint16(body[14:16])
			if format != 1 || channels != 1 || rate != Rate || bits != 16 {
				return nil, fmt.Errorf("want %d Hz mono 16-bit pcm, got format %d, %d channels, %d Hz, %d bits", Rate, format, channels, rate, bits)
			}
			formatOK = true
		case "data":
			if !formatOK {
				return nil, errors.New("data before fmt chunk")
			}
			samples := make([]int16, size/2)
			for j := range samples {
				samples[j] = int16(binary.LittleEndian.Uint16(body[2*j:]))
			}
			return samples, nil
		}
		i += 8 + size + size%2 // chunks are padded to an even size
	}
	return nil, errors.New("no data chunk")
}
