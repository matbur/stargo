package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
)

func TestStartupWiring(t *testing.T) {
	t.Parallel()

	database, err := chatdb.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.ApplySchema(room.Schema); err != nil {
		t.Fatalf("ApplySchema() error = %v", err)
	}

	app := newApp(database, relay.New[struct{}](), stargo.WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })

	lobby := httptest.NewRecorder()
	app.ServeHTTP(lobby, httptest.NewRequest(http.MethodGet, "/", nil))
	if lobby.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", lobby.Code)
	}
	if body := lobby.Body.String(); !strings.Contains(body, "No rooms yet") {
		t.Fatalf("lobby missing the empty state: %s", body)
	}

	missing := httptest.NewRecorder()
	app.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/rooms/nope", nil))
	if missing.Code != http.StatusFound {
		t.Fatalf("GET /rooms/nope status = %d, want 302", missing.Code)
	}
}
