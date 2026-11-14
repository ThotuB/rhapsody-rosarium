// Package listen keeps track of who is tuned in to which piano. Clients say so again every few
// seconds while they listen; one that stops saying it is dropped after a while, so a closed tab
// needs no goodbye.
package listen

import (
	"sync"
	"time"
)

// a client counts as listening this long after it last said so; clients repeat it every 5 s
const expireAfter = 15 * time.Second

// Audience is who is listening to one piano.
type Audience struct {
	Listeners int  `json:"listeners"` // everyone tuned in, the museum included
	Museum    bool `json:"museum"`    // the museum installation is one of them
}

// Table is safe to use from several goroutines. The zero value is ready to use.
type Table struct {
	Now func() time.Time // the clock, swapped in tests; nil means time.Now

	mu      sync.Mutex
	clients map[string]entry // by client id
}

type entry struct {
	piano  string
	museum bool
	seen   time.Time
}

func (t *Table) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// Mark records that the client is listening to the piano right now, or to nothing if piano is "".
func (t *Table) Mark(client, piano string, museum bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.clients == nil {
		t.clients = map[string]entry{}
	}
	if piano == "" {
		delete(t.clients, client)
		return
	}
	t.clients[client] = entry{piano: piano, museum: museum, seen: t.now()}
}

// Of returns who is listening to one piano.
func (t *Table) Of(piano string) Audience {
	return t.All()[piano]
}

// All returns who is listening, by piano. Pianos nobody listens to are left out.
func (t *Table) All() map[string]Audience {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	all := map[string]Audience{}
	for client, e := range t.clients {
		if now.Sub(e.seen) > expireAfter {
			delete(t.clients, client)
			continue
		}
		a := all[e.piano]
		a.Listeners++
		a.Museum = a.Museum || e.museum
		all[e.piano] = a
	}
	return all
}
