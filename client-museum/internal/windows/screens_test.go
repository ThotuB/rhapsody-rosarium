package windows

import "testing"

func TestLaptopAndProjector(t *testing.T) {
	for _, c := range []struct {
		connectors        []string
		laptop, projector int
		ok                bool
	}{
		{[]string{"eDP-1"}, 0, 0, false},
		{[]string{"eDP-1", "HDMI-A-1"}, 0, 1, true},
		{[]string{"HDMI-A-1", "eDP-1"}, 1, 0, true},
		{[]string{"DP-1", "HDMI-A-1"}, 0, 1, true}, // nothing built in: the first one
		{[]string{"DP-2", "HDMI-A-1", "eDP-1"}, 2, 0, true},
	} {
		laptop, projector, ok := laptopAndProjector(c.connectors)
		if laptop != c.laptop || projector != c.projector || ok != c.ok {
			t.Errorf("%v: got %d, %d, %v, want %d, %d, %v", c.connectors, laptop, projector, ok, c.laptop, c.projector, c.ok)
		}
	}
}
