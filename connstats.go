package stargo

import (
	"net"
	"net/http"
	"sync"
)

// connStats classifies connection transitions for the shutdown summary.
type connStats struct {
	mu           sync.Mutex
	states       map[net.Conn]http.ConnState
	open         int
	shuttingDown bool
	idleClosed   int
	forceClosed  int
}

func newConnStats() *connStats {
	return &connStats{states: make(map[net.Conn]http.ConnState)}
}

// observe records one connection transition. It may be chained after a
// user-supplied ConnState hook.
func (s *connStats) observe(conn net.Conn, state http.ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, known := s.states[conn]
	switch state {
	case http.StateNew:
		if !known {
			s.open++
			s.states[conn] = state
		}
	case http.StateActive, http.StateIdle:
		if !known {
			s.open++
		}
		s.states[conn] = state
	case http.StateHijacked, http.StateClosed:
		if !known {
			return
		}
		delete(s.states, conn)
		s.open--
		if state == http.StateClosed && s.shuttingDown {
			switch previous {
			case http.StateIdle:
				s.idleClosed++
			case http.StateActive:
				s.forceClosed++
			}
		}
	}
}

func (s *connStats) beginShutdown() {
	s.mu.Lock()
	s.shuttingDown = true
	s.mu.Unlock()
}

func (s *connStats) openCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.open
}

// shutdownCounts is the snapshot reported after shutdown.
type shutdownCounts struct {
	Open        int
	IdleClosed  int
	ForceClosed int
	Aborted     int
	Stale       int
	Cancelled   int
}

func (s *connStats) snapshot(open, cancelled, aborted int) shutdownCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := shutdownCounts{
		Open:        open,
		IdleClosed:  s.idleClosed,
		ForceClosed: s.forceClosed,
		Aborted:     aborted,
		Stale:       s.open,
		Cancelled:   cancelled,
	}
	if counts.Aborted > counts.ForceClosed {
		counts.ForceClosed = counts.Aborted
	}
	return counts
}
