package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"rhapsody-rosarium/museum/internal/lights"
)

// Config is the installation's fixed settings, from museum-bulbs.json in the folder the app runs from.
// Flags given on the command line override it.
type Config struct {
	Server string `json:"server"` // the rhapsody rosarium server
	Lights string `json:"lights"` // the led strips' board: a port like /dev/ttyACM0, "auto", "log" or "off"
	// the piano each strip follows, in strip order (bulb 1 first); null or "" leaves a strip off.
	// empty or missing: the server's first pianos
	Strips []string `json:"strips"`
}

var defaults = Config{Server: "http://localhost:8080", Lights: "auto"}

// loadConfig reads the config at path. A missing file is fine if it's only the default path, then
// it's the defaults; one asked for with -config has to be there.
func loadConfig(path string, asked bool) (Config, error) {
	c := defaults
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

// overrideWithFlags replaces what the config says with the flags given on the command line.
func (c *Config) overrideWithFlags(server, lightsPort, strips string) {
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
