package main

import (
	"net/http"
	"testing"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
	stargotest "github.com/matbur/stargo/testing"
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

	application := newApp(database, relay.New[struct{}](), stargo.WithTracer(nil))
	t.Cleanup(func() { _ = application.Close() })

	client := stargotest.NewClient(t, application, stargotest.WithoutRedirects())
	client.Get("/").
		AssertStatus(http.StatusOK).
		AssertContains("No rooms yet")
	client.Get("/rooms/nope").
		AssertStatus(http.StatusFound)
}
