package routes_test

import (
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/routes"
	"github.com/matbur/stargo/route"
)

func TestRouteHandles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handle  route.Route
		method  string
		pattern string
		href    string
	}{
		{"lobby", routes.Lobby, "GET", "GET /", "/"},
		{"lobby live", routes.LobbyLive, "GET", "GET /live", "/live"},
		{"create room", routes.CreateRoom, "POST", "POST /rooms", "/rooms"},
		{"room", routes.Room, "GET", "GET /rooms/{room_id}", "/rooms/design"},
		{"room live", routes.RoomLive, "GET", "GET /rooms/{room_id}/live", "/rooms/design/live"},
		{"send", routes.SendMessage, "POST", "POST /rooms/{room_id}/send", "/rooms/design/send"},
		{"typing", routes.SetTyping, "POST", "POST /rooms/{room_id}/typing", "/rooms/design/typing"},
		{"delete", routes.DeleteRoom, "DELETE", "DELETE /rooms/{room_id}", "/rooms/design"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.handle.Method(); got != test.method {
				t.Fatalf("Method() = %q, want %q", got, test.method)
			}
			if got := test.handle.Pattern(); got != test.pattern {
				t.Fatalf("Pattern() = %q, want %q", got, test.pattern)
			}

			var href string
			if test.handle.HasParams() {
				href = test.handle.Href("design")
			} else {
				href = test.handle.Href()
			}
			if href != test.href {
				t.Fatalf("Href() = %q, want %q", href, test.href)
			}
		})
	}
}
