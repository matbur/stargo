package room_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo/examples/chat-room/app/features/room"
)

const identitySignals = `{"user_id":"u1","username":"HappyFox","color":"#000"}`

func roomSignalURL(base, path, payload string) string {
	return base + path + "?datastar=" + url.QueryEscape(payload)
}

// openStream starts a live request and returns its reader. A deadline on the
// request context bounds every read.
func openStream(t *testing.T, target string) (*bufio.Reader, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		cancel()
		t.Fatalf("NewRequest() error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		cancel()
		t.Fatalf("Do() error = %v", err)
	}
	t.Cleanup(func() {
		_ = response.Body.Close()
		cancel()
	})

	return bufio.NewReader(response.Body), cancel
}

func nextEvent(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	var event strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		if line == "\n" {
			return event.String()
		}
		event.WriteString(line)
	}
}

// nextEventContaining skips patches the test does not care about, such as the
// presence patch produced by the watcher's own join.
func nextEventContaining(t *testing.T, reader *bufio.Reader, needle string) string {
	t.Helper()

	for i := 0; i < 10; i++ {
		event := nextEvent(t, reader)
		if strings.Contains(event, needle) {
			return event
		}
	}
	t.Fatalf("no event contained %q", needle)
	return ""
}

func TestLiveJoinsAndPatches(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	reader, _ := openStream(t, roomSignalURL(server.URL, "/rooms/"+added.ID+"/live", identitySignals))
	event := nextEvent(t, reader)

	for _, want := range []string{"datastar-patch-elements", "selector #room-live", "Design Chat", "HappyFox"} {
		if !strings.Contains(event, want) {
			t.Fatalf("first patch missing %q: %s", want, event)
		}
	}

	exists, err := room.UserExists(app.database, "u1", added.ID)
	if err != nil {
		t.Fatalf("UserExists() error = %v", err)
	}
	if !exists {
		t.Fatal("join did not register presence")
	}
}

func TestLiveReceivesMessageAndTyping(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	reader, _ := openStream(t, roomSignalURL(server.URL, "/rooms/"+added.ID+"/live", identitySignals))
	_ = nextEvent(t, reader) // initial patch

	if err := room.AddMessage(app.database, room.Message{
		ID: "m1", RoomID: added.ID, UserID: "u1", Username: "HappyFox",
		Color: "#000", Text: "hello watcher", Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	app.relay.Publish(room.MessageSubject(added.ID), struct{}{})

	messageEvent := nextEventContaining(t, reader, "hello watcher")
	if !strings.Contains(messageEvent, "selector #room-live") {
		t.Fatalf("message patch missing selector: %s", messageEvent)
	}

	if err := room.AddUser(app.database, room.User{
		ID: "u2", RoomID: added.ID, Username: "BraveOwl", Color: "#111", Typing: true,
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	app.relay.Publish(room.TypingSubject(added.ID), struct{}{})

	typingEvent := nextEventContaining(t, reader, "is typing")
	if !strings.Contains(typingEvent, "BraveOwl is typing") {
		t.Fatalf("typing patch missing text: %s", typingEvent)
	}
}

func TestLivePresenceForOtherWatchers(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	watcher, _ := openStream(t, roomSignalURL(server.URL, "/rooms/"+added.ID+"/live", identitySignals))
	_ = nextEvent(t, watcher) // initial patch

	second := `{"user_id":"u2","username":"BraveOwl","color":"#111"}`
	_, stopSecond := openStream(t, roomSignalURL(server.URL, "/rooms/"+added.ID+"/live", second))

	joinEvent := nextEventContaining(t, watcher, "2 online")
	if !strings.Contains(joinEvent, "BraveOwl") {
		t.Fatalf("join patch missing the new user: %s", joinEvent)
	}

	stopSecond()

	leftEvent := nextEventContaining(t, watcher, "1 online")
	if strings.Contains(leftEvent, "BraveOwl") {
		t.Fatalf("leave patch still lists the user: %s", leftEvent)
	}
}

func TestLiveNavigatesOnDeletion(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	reader, _ := openStream(t, roomSignalURL(server.URL, "/rooms/"+added.ID+"/live", identitySignals))
	_ = nextEvent(t, reader) // initial patch

	if _, err := room.DeleteRoom(app.database, added.ID); err != nil {
		t.Fatalf("DeleteRoom() error = %v", err)
	}
	app.relay.Publish(room.DeletedSubject(added.ID), struct{}{})

	event := nextEventContaining(t, reader, "window.location")
	if !strings.Contains(event, `window.location = "/"`) {
		t.Fatalf("deletion did not navigate to the lobby: %s", event)
	}
}

func TestLiveUnknownRoomNavigates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	reader, _ := openStream(t, roomSignalURL(server.URL, "/rooms/nope/live", identitySignals))
	event := nextEvent(t, reader)
	if !strings.Contains(event, `window.location = "/"`) {
		t.Fatalf("unknown room did not navigate: %s", event)
	}
}

func TestLiveMissingIdentityNavigates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	reader, _ := openStream(t, server.URL+"/rooms/"+added.ID+"/live")
	event := nextEvent(t, reader)
	if !strings.Contains(event, `window.location = "/rooms/design-chat"`) {
		t.Fatalf("missing identity did not navigate: %s", event)
	}

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("rejected stream registered presence: %+v", users)
	}
}
