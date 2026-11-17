// Command chat-room is the multi-feature Stargo example: a lobby, per-room
// chat pages, SQLite persistence, and Datastar live updates.
package main

import (
	"log"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/examples/chat-room/app/config"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/lobby"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
)

func newApp(database *chatdb.DB, hub *relay.Relay[struct{}], opts ...stargo.Option) *stargo.App {
	app := stargo.New(opts...)
	lobby.Page{Database: database, Relay: hub}.Register(app)
	room.Page{Database: database, Relay: hub}.Register(app)
	return app
}

func main() {
	cfg := config.FromEnv()

	database, err := chatdb.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	if err := database.ApplySchema(room.Schema); err != nil {
		log.Fatal(err)
	}

	app := newApp(database, relay.New[struct{}]())
	defer func() { _ = app.Close() }()

	if err := app.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
