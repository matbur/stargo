package room

import (
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo/examples/chat-room/app/common"
	stargotest "github.com/matbur/stargo/testing"
)

func TestLiveViewPatchRoot(t *testing.T) {
	t.Parallel()

	chatRoom := Room{ID: "design", Title: "Design Chat", Description: "UI talk"}
	html := stargotest.Render(t, LiveView(chatRoom, "u1", nil, nil))

	for _, want := range []string{
		`id="room-live"`,
		"Design Chat",
		"UI talk",
		`href="/"`,
		`id="messages"`,
		"No messages yet. Say hello!",
		`id="input-form"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("room view missing %q", want)
		}
	}
}

func TestMessageOwnStylingAndOrder(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 1, 2, 15, 4, 5, 0, time.Local).UnixMilli()
	messages := []Message{
		{ID: "m1", UserID: "u1", Username: "HappyFox", Color: "#111", Text: "first", Timestamp: timestamp},
		{ID: "m2", UserID: "u2", Username: "BraveOwl", Color: "#222", Text: "second", Timestamp: timestamp + 60_000},
	}

	html := stargotest.Render(t, LiveView(Room{ID: "design", Title: "Design"}, "u1", messages, nil))

	if !strings.Contains(html, `class="message own"`) {
		t.Fatalf("own message not styled: %s", html)
	}
	if !strings.Contains(html, `class="message"`) {
		t.Fatalf("other message missing default class: %s", html)
	}
	if !strings.Contains(html, `data-msg-id="m1"`) || !strings.Contains(html, `data-msg-id="m2"`) {
		t.Fatalf("message ids missing: %s", html)
	}
	if strings.Index(html, "first") > strings.Index(html, "second") {
		t.Fatal("messages are not oldest first")
	}
	if !strings.Contains(html, "15:04") {
		t.Fatalf("timestamp missing: %s", html)
	}
	if !strings.Contains(html, `data-on:load="setTimeout(() =&gt; el.scrollTop = el.scrollHeight, 10)"`) {
		t.Fatal("scroll on load missing")
	}
}

func TestTypingSummaryVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		users []User
		want  string
	}{
		{"none", nil, ""},
		{"one", []User{{Username: "HappyFox", Typing: true}}, "HappyFox is typing"},
		{"two", []User{{Username: "HappyFox", Typing: true}, {Username: "BraveOwl", Typing: true}},
			"HappyFox and BraveOwl are typing"},
		{"many", []User{
			{Username: "HappyFox", Typing: true},
			{Username: "BraveOwl", Typing: true},
			{Username: "MightyBear", Typing: true},
		}, "HappyFox and 2 others are typing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := typingText(test.users); got != test.want {
				t.Fatalf("typingText() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTypingIndicatorHidesOwnTyping(t *testing.T) {
	t.Parallel()

	html := stargotest.Render(t, LiveView(Room{ID: "design"}, "u1", nil, []User{
		{ID: "u1", Username: "HappyFox", Typing: true},
	}))
	if !strings.Contains(html, `class="typing-indicator hidden"`) {
		t.Fatalf("own typing should be hidden: %s", html)
	}

	html = stargotest.Render(t, LiveView(Room{ID: "design"}, "u1", nil, []User{
		{ID: "u2", Username: "BraveOwl", Typing: true},
	}))
	if !strings.Contains(html, "BraveOwl is typing") {
		t.Fatalf("other typing missing: %s", html)
	}
}

func TestOnlineUsersAvatars(t *testing.T) {
	t.Parallel()

	users := make([]User, 0, 10)
	for i := 0; i < 10; i++ {
		users = append(users, User{
			ID:       string(rune('a' + i)),
			Username: string(rune('A'+i)) + "name",
			Color:    "#000",
		})
	}

	html := stargotest.Render(t, LiveView(Room{ID: "design"}, "u1", nil, users))
	for _, want := range []string{"10 online", `class="avatar"`, `class="avatar more"`, "+2"} {
		if !strings.Contains(html, want) {
			t.Fatalf("online users missing %q", want)
		}
	}

	empty := stargotest.Render(t, LiveView(Room{ID: "design"}, "u1", nil, nil))
	if !strings.Contains(empty, `id="online" class="online-users"`) &&
		!strings.Contains(empty, `class="online-users" id="online"`) {
		t.Fatalf("empty online users missing: %s", empty)
	}
}

func TestViewCarriesSignalsAndLiveInit(t *testing.T) {
	t.Parallel()

	identity := common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#000"}
	html := stargotest.Render(t, View(identity, Room{ID: "design"}, nil, nil, "/rooms/design/live"))

	for _, want := range []string{
		`id="room"`,
		"data-signals__ifmissing",
		"abc12345",
		"sessionStorage",
		`@get(&#39;/rooms/design/live&#39;`,
		`data-bind="message"`,
		`data-on:input`,
		`/rooms/design/typing`,
		`/rooms/design/send`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("room page missing %q", want)
		}
	}
}
