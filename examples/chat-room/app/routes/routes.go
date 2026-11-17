// Package routes declares every chat-room route handle.
//
// It is a leaf package: both features import it for registration and links, so
// neither feature needs to import the other.
package routes

import "github.com/matbur/stargo/route"

var (
	// Lobby is the room list page.
	Lobby = route.Get("/")
	// LobbyLive is the lobby SSE stream.
	LobbyLive = route.Get("/live")

	// CreateRoom creates a room from dialog signals.
	CreateRoom = route.Post("/rooms")

	// Room is one room page.
	Room = route.Get("/rooms/{room_id}")
	// RoomLive is one room's SSE stream.
	RoomLive = route.Get("/rooms/{room_id}/live")
	// SendMessage stores a message.
	SendMessage = route.Post("/rooms/{room_id}/send")
	// SetTyping updates the sender's typing flag.
	SetTyping = route.Post("/rooms/{room_id}/typing")

	// DeleteRoom removes a room and its data.
	DeleteRoom = route.Delete("/rooms/{room_id}")
)
