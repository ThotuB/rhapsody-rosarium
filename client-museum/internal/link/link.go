// Package link is the museum's connection to the server: it reads the pianos' state, downloads
// their audio segments and says which one the museum listens to.
package link

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Segment mirrors a segment in the server's GET /api/state.
type Segment struct {
	URL string    `json:"url"`
	At  time.Time `json:"at"`
	Seq *int64    `json:"seq"` // the node's window number, nil if it doesn't send them
}

// Piano mirrors a piano in the server's GET /api/state.
type Piano struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Lat        float64    `json:"lat"`
	Lng        float64    `json:"lng"`
	State      string     `json:"state"`
	Note       *string    `json:"note"`
	LastSeen   *time.Time `json:"lastSeen"`
	LastPlayed *time.Time `json:"lastPlayed"`
	Segments   []Segment  `json:"segments"`  // oldest first
	Listeners  int        `json:"listeners"` // clients tuned in to it, the museum included
	Museum     bool       `json:"museum"`    // the museum is one of them
}

// State mirrors the server's GET /api/state.
type State struct {
	Now    time.Time `json:"now"`
	Pianos []Piano   `json:"pianos"`
}

// Client talks to one server, e.g. http://localhost:8080.
type Client struct {
	server string
	http   *http.Client
}

func New(server string) *Client {
	return &Client{server: strings.TrimRight(server, "/"), http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) State(ctx context.Context) (State, error) {
	var s State
	body, err := c.get(ctx, "/api/state")
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return s, fmt.Errorf("/api/state: %w", err)
	}
	return s, nil
}

// Download fetches a segment's audio, a wav file.
func (c *Client) Download(ctx context.Context, seg Segment) ([]byte, error) {
	return c.get(ctx, seg.URL)
}

// Listening tells the server the museum is tuned in to the piano, "" once it stopped. The server
// forgets it after 15 s, so it has to be repeated while listening.
func (c *Client) Listening(ctx context.Context, piano string) error {
	form := url.Values{"client": {"museum"}, "piano": {piano}, "museum": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.server+"/api/listening", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("POST /api/listening: %s", res.Status)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.server+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", path, res.Status)
	}
	return io.ReadAll(res.Body)
}
