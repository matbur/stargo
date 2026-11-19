package lobby_test

import (
	"strings"
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/common"
	"github.com/matbur/stargo/examples/chat-room/app/features/lobby"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	stargotest "github.com/matbur/stargo/testing"
)

func TestLiveViewEmptyState(t *testing.T) {
	t.Parallel()

	html := stargotest.Render(t, lobby.LiveView(nil, nil))
	checks := []string{
		`id="lobby-live"`,
		"No rooms yet",
		"Stario Chat",
		`id="create-room-dialog"`,
		`data-bind="room_title"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Fatalf("empty lobby missing %q", want)
		}
	}
}

func TestLiveViewRoomCards(t *testing.T) {
	t.Parallel()

	rooms := []room.Room{
		{ID: "design", Title: "Design Chat", Description: "UI talk"},
		{ID: "general", Title: "General", Description: "Welcome"},
	}
	html := stargotest.Render(t, lobby.LiveView(rooms, map[string]int{"design": 3}))

	checks := []string{
		`href="/rooms/design"`,
		`href="/rooms/general"`,
		"Design Chat",
		"UI talk",
		"3 online",
		"0 online",
		`@delete(&#39;/rooms/design&#39;)`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Fatalf("room list missing %q in %s", want, html)
		}
	}
}

func TestViewCarriesSignalsAndLiveInit(t *testing.T) {
	t.Parallel()

	identity := common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#000"}
	html := stargotest.Render(t, lobby.View(identity, nil, nil, "/live"))

	checks := []string{
		`id="lobby"`,
		"data-signals__ifmissing",
		"abc12345",
		"HappyFox",
		"room_title",
		"sessionStorage",
		`@get(&#39;/live&#39;, {retry: &#39;always&#39;})`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Fatalf("lobby page missing %q", want)
		}
	}
}
