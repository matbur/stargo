package room_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	stargotest "github.com/matbur/stargo/testing"
)

const identitySignals = `{"user_id":"u1","username":"HappyFox","color":"#000"}`

func signalURL(path, payload string) string {
	return path + "?datastar=" + url.QueryEscape(payload)
}

// openStream starts a live request with a deadline that bounds every read.
func openStream(t *testing.T, app handlerApp, target string) *stargotest.Stream {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	stream := app.client.Open(app.client.Request(http.MethodGet, target).WithContext(ctx))
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

// nextPatchContaining skips patches the test does not care about, such as the
// presence patch produced by the watcher's own join.
func nextPatchContaining(t *testing.T, stream *stargotest.Stream, needle string) stargotest.Patch {
	t.Helper()

	for range 10 {
		patch, ok := stream.NextPatch()
		if !ok {
			t.Fatalf("stream ended before a patch contained %q", needle)
		}
		if strings.Contains(patch.Elements, needle) {
			return patch
		}
	}
	t.Fatalf("no patch contained %q", needle)
	return stargotest.Patch{}
}

func TestLiveJoinsAndPatches(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	stream := openStream(t, app, signalURL("/rooms/"+added.ID+"/live", identitySignals))

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}
	if patch.Kind != stargotest.PatchElements ||
		patch.Selector != "#room-live" ||
		!strings.Contains(patch.Elements, "Design Chat") ||
		!strings.Contains(patch.Elements, "HappyFox") {
		t.Fatalf("first patch = %+v", patch)
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
	stream := openStream(t, app, signalURL("/rooms/"+added.ID+"/live", identitySignals))
	if _, ok := stream.NextPatch(); !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}

	if err := room.AddMessage(app.database, room.Message{
		ID: "m1", RoomID: added.ID, UserID: "u1", Username: "HappyFox",
		Color: "#000", Text: "hello watcher", Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	app.relay.Publish(room.MessageSubject(added.ID), struct{}{})

	messagePatch := nextPatchContaining(t, stream, "hello watcher")
	if messagePatch.Selector != "#room-live" {
		t.Fatalf("message patch selector = %q", messagePatch.Selector)
	}

	if err := room.AddUser(app.database, room.User{
		ID: "u2", RoomID: added.ID, Username: "BraveOwl", Color: "#111", Typing: true,
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	app.relay.Publish(room.TypingSubject(added.ID), struct{}{})

	typingPatch := nextPatchContaining(t, stream, "is typing")
	if !strings.Contains(typingPatch.Elements, "BraveOwl is typing") {
		t.Fatalf("typing patch = %+v", typingPatch)
	}
}

func TestLivePresenceForOtherWatchers(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	watcher := openStream(t, app, signalURL("/rooms/"+added.ID+"/live", identitySignals))
	if _, ok := watcher.NextPatch(); !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}

	second := openStream(t, app,
		signalURL("/rooms/"+added.ID+"/live", `{"user_id":"u2","username":"BraveOwl","color":"#111"}`))

	joinPatch := nextPatchContaining(t, watcher, "2 online")
	if !strings.Contains(joinPatch.Elements, "BraveOwl") {
		t.Fatalf("join patch = %+v", joinPatch)
	}

	if err := second.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	leftPatch := nextPatchContaining(t, watcher, "1 online")
	if strings.Contains(leftPatch.Elements, "BraveOwl") {
		t.Fatalf("leave patch still lists the user: %+v", leftPatch)
	}
}

func TestLiveNavigatesOnDeletion(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	stream := openStream(t, app, signalURL("/rooms/"+added.ID+"/live", identitySignals))
	if _, ok := stream.NextPatch(); !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}

	if _, err := room.DeleteRoom(app.database, added.ID); err != nil {
		t.Fatalf("DeleteRoom() error = %v", err)
	}
	app.relay.Publish(room.DeletedSubject(added.ID), struct{}{})

	patch := nextPatchContaining(t, stream, "window.location")
	if !strings.Contains(patch.Elements, `window.location = "/"`) {
		t.Fatalf("deletion did not navigate to the lobby: %+v", patch)
	}
}

func TestLiveUnknownRoomNavigates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	stream := openStream(t, app, signalURL("/rooms/nope/live", identitySignals))

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no navigation patch")
	}
	if !strings.Contains(patch.Elements, `window.location = "/"`) {
		t.Fatalf("unknown room did not navigate: %+v", patch)
	}
}

func TestLiveMissingIdentityNavigates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	stream := openStream(t, app, "/rooms/"+added.ID+"/live")

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no navigation patch")
	}
	if !strings.Contains(patch.Elements, `window.location = "/rooms/design-chat"`) {
		t.Fatalf("missing identity did not navigate: %+v", patch)
	}

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("rejected stream registered presence: %+v", users)
	}
}
