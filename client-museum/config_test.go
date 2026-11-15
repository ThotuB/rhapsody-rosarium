package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "museum-bulbs.json")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "museum-bulbs.json")
	if c, err := loadConfig(missing, false); err != nil || c.Server != defaults.Server || c.Lights != "auto" || c.Strips != nil {
		t.Errorf("no file at the default path should give the defaults: %+v, %v", c, err)
	}
	if _, err := loadConfig(missing, true); err == nil {
		t.Error("a config asked for with -config has to exist")
	}

	c, err := loadConfig(writeConfig(t, `{"lights": "/dev/ttyACM0", "strips": ["piano-2", null, "", "piano-4"]}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server != defaults.Server {
		t.Errorf("a key left out should keep its default: server %q", c.Server)
	}
	if c.Lights != "/dev/ttyACM0" || !slices.Equal(c.Strips, []string{"piano-2", "", "", "piano-4"}) {
		t.Errorf("null and \"\" both leave a strip off, in place: %+v", c)
	}

	for name, text := range map[string]string{
		"misspelt key":    `{"strip": ["piano-2"]}`,
		"too many strips": `{"strips": ["a", "b", "c", "d", "e", "f", "g"]}`,
		"not json":        `strips: piano-2`,
	} {
		if _, err := loadConfig(writeConfig(t, text), true); err == nil {
			t.Errorf("%s: should be an error", name)
		}
	}
}

func TestShippedConfigLoads(t *testing.T) {
	c, err := loadConfig("museum-bulbs.json", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range c.Strips {
		if id != "" && !strings.HasPrefix(id, "piano-") {
			t.Errorf("unexpected piano id %q", id)
		}
	}
}
