package museum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"

	"rhapsody-rosarium/museum/internal/player"
)

// Used only by the feed goroutine, never by the sound-card callback. An absent
// helper has a short cooldown so uploads keep playing without repeated waits.
type analyzer struct {
	client  *http.Client
	url     string
	retryAt time.Time
}

func newAnalyzer() *analyzer {
	return &analyzer{client: &http.Client{Timeout: 45 * time.Second}, url: "http://127.0.0.1:8765/transcribe"}
}

func (a *analyzer) analyze(ctx context.Context, wav []byte) ([]player.Note, string) {
	const unavailable = "Note recognition unavailable — start the local AI helper on port 8765."
	if time.Now().Before(a.retryAt) {
		return nil, unavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(wav))
	if err != nil {
		return nil, unavailable
	}
	req.Header.Set("Content-Type", "audio/wav")
	res, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ""
		}
		a.retryAt = time.Now().Add(10 * time.Second)
		return nil, unavailable
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		a.retryAt = time.Now().Add(10 * time.Second)
		return nil, fmt.Sprintf("Note recognition failed (%d). Check the AI helper terminal.", res.StatusCode)
	}
	var result struct {
		Notes    []player.Note `json:"notes"`
		Duration float64       `json:"duration"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&result); err != nil {
		return nil, "Invalid response from note recognition."
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if !finite(result.Duration) || result.Duration <= 0 || result.Duration > 180 {
		return nil, "Invalid recording duration from note recognition."
	}
	for _, n := range result.Notes {
		if n.Pitch < 21 || n.Pitch > 108 || !finite(n.Start) || !finite(n.Duration) || !finite(n.Velocity) ||
			n.Start < 0 || n.Duration <= 0 || n.Start+n.Duration > result.Duration+0.01 || n.Velocity < 0 || n.Velocity > 127 {
			return nil, "Invalid notes from note recognition."
		}
	}
	sort.SliceStable(result.Notes, func(i, j int) bool { return result.Notes[i].Start < result.Notes[j].Start })
	return result.Notes, ""
}
