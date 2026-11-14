package listen

import (
	"testing"
	"time"
)

func TestTable(t *testing.T) {
	clock := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	var tb Table
	tb.Now = func() time.Time { return clock }

	if a := tb.Of("a"); a.Listeners != 0 || a.Museum {
		t.Errorf("nobody yet: %+v", a)
	}

	tb.Mark("web-1", "a", false)
	tb.Mark("web-2", "a", false)
	tb.Mark("museum", "b", true)
	if a := tb.Of("a"); a.Listeners != 2 || a.Museum {
		t.Errorf("two web listeners on a: %+v", a)
	}
	if b := tb.Of("b"); b.Listeners != 1 || !b.Museum {
		t.Errorf("the museum on b: %+v", b)
	}

	tb.Mark("web-2", "b", false) // switched stations
	if a, b := tb.Of("a"), tb.Of("b"); a.Listeners != 1 || b.Listeners != 2 || !b.Museum {
		t.Errorf("after web-2 switched to b: a %+v, b %+v", a, b)
	}

	tb.Mark("web-1", "", false) // stopped
	if a := tb.Of("a"); a.Listeners != 0 {
		t.Errorf("after web-1 stopped: %+v", a)
	}

	clock = clock.Add(10 * time.Second)
	tb.Mark("museum", "b", true) // still listening, web-2 went quiet
	clock = clock.Add(10 * time.Second)
	if b := tb.Of("b"); b.Listeners != 1 || !b.Museum {
		t.Errorf("web-2 should have expired, the museum not: %+v", b)
	}
	if all := tb.All(); len(all) != 1 {
		t.Errorf("only b has listeners: %v", all)
	}
}
