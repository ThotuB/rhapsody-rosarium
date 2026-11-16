// Package config is the museum installation's fixed settings: the server, the lights board and which
// piano each bulb follows.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"rhapsody-rosarium/museum/internal/lights"
)

// Config is the installation's fixed settings, from config/museum-bulbs.json (-config), relative to
// the folder the app runs from.
// Flags given on the command line override it.
type Config struct {
	Server string `json:"server"` // the rhapsody rosarium server
	Lights string `json:"lights"` // the led strips' board: a port like /dev/ttyACM0, "auto", "log" or "off"
	// the piano each strip follows, in strip order (bulb 1 first); null or "" leaves a strip off.
	// empty or missing: the server's first pianos
	Strips []string `json:"strips"`
}

// Defaults is what a setting left out of the file is.
var Defaults = Config{Server: "http://localhost:8080", Lights: "auto"}

// Load reads the config at path. A missing file is fine if it's only the default path, then
// it's the defaults; one asked for with -config has to be there.
func Load(path string, asked bool) (Config, error) {
	c := Defaults
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !asked {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields() // a misspelt key would otherwise be silently ignored
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	if len(c.Strips) > lights.Strips {
		return c, fmt.Errorf("%s: %d strips, the board drives at most %d", path, len(c.Strips), lights.Strips)
	}
	return c, nil
}

// OverrideWithFlags replaces what the config says with the flags given on the command line.
func (c *Config) OverrideWithFlags(server, lightsPort, strips string) {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "server":
			c.Server = server
		case "lights":
			c.Lights = lightsPort
		case "strips":
			c.Strips = pianoList(strips)
		}
	})
}

// pianoList splits "piano-2,piano-3" into ids, nil for "". An empty slot ("piano-2,,piano-4") stays
// in place as "", so the strips after it keep their pianos and that strip stays off.
func pianoList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var ids []string
	for id := range strings.SplitSeq(s, ",") {
		ids = append(ids, strings.TrimSpace(id))
	}
	return ids
}
