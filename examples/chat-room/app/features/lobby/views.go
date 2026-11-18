// Package lobby is the room list landing page.
package lobby

import (
	"encoding/json"
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	ds "github.com/matbur/stargo/datastar"
	"github.com/matbur/stargo/examples/chat-room/app/common"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/examples/chat-room/app/routes"
)

// LiveSelector identifies the element replaced by every live patch.
const LiveSelector = "#lobby-live"

// LiveView is the patch target for SSE. It excludes the page shell that owns
// the live connection, so patches never restart it.
func LiveView(rooms []room.Room, onlineCounts map[string]int) g.Node {
	return h.Div(
		g.Attr("id", "lobby-live"),
		h.Div(
			g.Attr("class", "lobby-header"),
			h.H1(g.Attr("class", "lobby-title"), g.Text("Stario Chat 🐾")),
			h.Button(
				g.Attr("type", "button"),
				g.Attr("class", "lobby-new-btn"),
				ds.On("click", "document.getElementById('create-room-dialog').showModal()"),
				g.Text("New room"),
			),
		),
		h.P(
			g.Attr("class", "lobby-lead"),
			g.Text("Pick a room — each has its own URL, messages, and presence."),
		),
		roomList(rooms, onlineCounts),
		createRoomDialog(),
	)
}

// View is the complete lobby page.
func View(identity common.Identity, rooms []room.Room, onlineCounts map[string]int, liveHref string) g.Node {
	return common.Page(
		h.Div(
			g.Attr("id", "lobby"),
			g.Attr("class", "lobby-container"),
			signalsAttribute(identity),
			ds.Init(liveInit(liveHref)),
			LiveView(rooms, onlineCounts),
		),
	)
}

func liveInit(liveHref string) string {
	return common.SessionInitScript() + "\n" + ds.Get(liveHref, ds.WithRetry(ds.RetryAlways))
}

func signalsAttribute(identity common.Identity) g.Node {
	payload := identity.Signals()
	payload["room_title"] = ""
	payload["room_description"] = ""
	return ds.Signals(payload, ds.WithIfMissing())
}

func roomList(rooms []room.Room, onlineCounts map[string]int) g.Node {
	if len(rooms) == 0 {
		return h.Ul(
			g.Attr("class", "room-list"),
			h.Li(h.P(g.Attr("class", "lobby-empty"), g.Text("No rooms yet — create one above."))),
		)
	}

	cards := make([]g.Node, 0, len(rooms))
	for _, chatRoom := range rooms {
		cards = append(cards, roomCard(chatRoom, onlineCounts[chatRoom.ID]))
	}
	return h.Ul(g.Attr("class", "room-list"), g.Group(cards))
}

func roomCard(chatRoom room.Room, online int) g.Node {
	confirmText := jsonString(fmt.Sprintf("Delete %s?", chatRoom.Title))
	deleteCall := ds.Delete(routes.DeleteRoom.Href(chatRoom.ID))
	return h.Li(
		h.Div(
			g.Attr("class", "room-card"),
			h.A(
				g.Attr("class", "room-card-link"),
				h.Href(routes.Room.Href(chatRoom.ID)),
				h.Span(g.Attr("class", "room-card-title"), g.Text(chatRoom.Title)),
				h.Span(g.Attr("class", "room-card-desc"), g.Text(chatRoom.Description)),
				h.Span(g.Attr("class", "room-card-meta"), g.Text(fmt.Sprintf("%d online", online))),
			),
			h.Button(
				g.Attr("type", "button"),
				g.Attr("class", "room-delete-btn"),
				g.Attr("title", "Delete room"),
				ds.On("click",
					"evt.stopPropagation(); if (confirm("+confirmText+")) { "+deleteCall+"; }"),
				g.Text("×"),
			),
		),
	)
}

func createRoomDialog() g.Node {
	createCall := ds.Post(routes.CreateRoom.Href())
	submit := "if ($room_title.trim()) { " + createCall + "; " +
		"$room_title = ''; $room_description = ''; el.closest('dialog').close(); }"

	return h.Dialog(
		g.Attr("id", "create-room-dialog"),
		g.Attr("class", "room-dialog"),
		h.Form(
			g.Attr("class", "room-dialog-form"),
			ds.On("submit", submit, ds.WithPrevent()),
			h.H2(g.Attr("class", "room-dialog-title"), g.Text("New room")),
			h.Label(
				g.Attr("class", "room-dialog-label"),
				g.Text("Name"),
				h.Input(
					g.Attr("type", "text"),
					g.Attr("class", "room-dialog-input"),
					g.Attr("placeholder", "e.g. Design"),
					g.Attr("autocomplete", "off"),
					ds.Bind("room_title"),
				),
			),
			h.Label(
				g.Attr("class", "room-dialog-label"),
				g.Text("Description"),
				h.Textarea(
					g.Attr("class", "room-dialog-textarea"),
					g.Attr("placeholder", "What is this room for?"),
					g.Attr("rows", "3"),
					ds.Bind("room_description"),
				),
			),
			h.Div(
				g.Attr("class", "room-dialog-actions"),
				h.Button(
					g.Attr("type", "button"),
					g.Attr("class", "room-dialog-cancel"),
					ds.On("click", "el.closest('dialog').close()"),
					g.Text("Cancel"),
				),
				h.Button(
					g.Attr("type", "submit"),
					g.Attr("class", "room-dialog-create"),
					ds.Attr("disabled", "!$room_title.trim()"),
					g.Text("Create"),
				),
			),
		),
	)
}

func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err) // strings always marshal
	}
	return string(encoded)
}
