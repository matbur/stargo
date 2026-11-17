package stargo

import (
	"net"
	"net/http"
	"testing"
)

func TestConnStatsClassifiesTransitions(t *testing.T) {
	t.Parallel()

	stats := newConnStats()
	idle, idlePeer := net.Pipe()
	defer idle.Close()
	defer idlePeer.Close()

	stats.observe(idle, http.StateNew)
	stats.observe(idle, http.StateActive)
	if got := stats.openCount(); got != 1 {
		t.Fatalf("open = %d, want 1", got)
	}

	stats.beginShutdown()
	stats.observe(idle, http.StateIdle)
	stats.observe(idle, http.StateClosed)

	active, activePeer := net.Pipe()
	defer active.Close()
	defer activePeer.Close()
	stats.observe(active, http.StateNew)
	stats.observe(active, http.StateActive)
	stats.observe(active, http.StateClosed)

	hijacked, hijackedPeer := net.Pipe()
	defer hijacked.Close()
	defer hijackedPeer.Close()
	stats.observe(hijacked, http.StateNew)
	stats.observe(hijacked, http.StateHijacked)

	counts := stats.snapshot(2, 3, 0)
	if counts.Open != 2 || counts.Cancelled != 3 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts.IdleClosed != 1 {
		t.Fatalf("idle_closed = %d, want 1", counts.IdleClosed)
	}
	if counts.ForceClosed != 1 {
		t.Fatalf("force_closed = %d, want 1", counts.ForceClosed)
	}
	if counts.Stale != 0 {
		t.Fatalf("stale = %d, want 0", counts.Stale)
	}
}
