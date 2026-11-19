package lobby_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/lobby"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
	stargotest "github.com/matbur/stargo/testing"
)

type testApp struct {
	app      *stargo.App
	client   *stargotest.Client
	database *chatdb.DB
	relay    *relay.Relay[struct{}]
}

func newTestApp(t *testing.T) testApp {
	t.Helper()

	database, err := chatdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.ApplySchema(room.Schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	hub := relay.New[struct{}]()
	app := stargo.New(stargo.WithTracer(nil))
	lobby.Page{Database: database, Relay: hub}.Register(app)
	t.Cleanup(func() { _ = app.Close() })

	return testApp{
		app:      app,
		client:   stargotest.NewClient(t, app),
		database: database,
		relay:    hub,
	}
}

func (a testApp) do(t *testing.T, method, target, body string) *stargotest.Response {
	t.Helper()

	var opts []stargotest.RequestOption
	if body != "" {
		opts = append(opts, stargotest.WithBody("application/json", []byte(body)))
	}
	return a.client.Do(a.client.Request(method, target, opts...))
}

func openStream(t *testing.T, app testApp, target string) *stargotest.Stream {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	stream := app.client.Open(app.client.Request(http.MethodGet, target).WithContext(ctx))
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func signalURL(path, payload string) string {
	return path + "?datastar=" + url.QueryEscape(payload)
}

func TestShowEmptyLobby(t *testing.T) {
	t.Parallel()

	newTestApp(t).do(t, http.MethodGet, "/", "").
		AssertStatus(http.StatusOK).
		AssertContains(
			"No rooms yet",
			`id="lobby-live"`,
			"data-signals__ifmissing",
			`@get(&#39;/live&#39;`,
		)
}

func TestShowListsRoomsWithPresence(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	added, err := room.AddRoom(app.database, "Design", "UI talk")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}
	if err := room.AddUser(app.database, room.User{
		ID: "u1", RoomID: added.ID, Username: "HappyFox", Color: "#000",
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}

	app.do(t, http.MethodGet, "/", "").
		AssertContains("Design", "UI talk", "1 online", `href="/rooms/design"`)
}

func TestShowMintsDistinctIdentities(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	first := app.client.Get("/").Text()
	second := app.client.Get("/").Text()
	if first == second {
		t.Fatal("two lobby pages used the same identity")
	}
}

func TestCreateRoomNavigates(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	changed, err := app.relay.Subscribe(context.Background(), lobby.Changed)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer changed.Close()

	app.do(t, http.MethodPost, "/rooms",
		`{"room_title":"Design Chat","room_description":"UI talk"}`).
		AssertStatus(http.StatusOK).
		AssertHeader("Content-Type", "text/event-stream").
		AssertContains(`"/rooms/design-chat"`)

	stored, err := room.GetRoom(app.database, "design-chat")
	if err != nil {
		t.Fatalf("GetRoom() error = %v", err)
	}
	if stored == nil || stored.Title != "Design Chat" || stored.Description != "UI talk" {
		t.Fatalf("stored room = %+v", stored)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if subject, _, err := changed.Next(ctx); err != nil || subject != lobby.Changed {
		t.Fatalf("relay Next() = %q, %v; want %q", subject, err, lobby.Changed)
	}
}

func TestCreateRoomDefaultsAndTruncates(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	longTitle := strings.Repeat("t", lobby.MaxTitleLength+10)
	app.do(t, http.MethodPost, "/rooms",
		`{"room_title":"`+longTitle+`","room_description":"  "}`)

	rooms, err := room.ListRooms(app.database)
	if err != nil {
		t.Fatalf("ListRooms() error = %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("rooms = %d, want 1", len(rooms))
	}
	if want := strings.Repeat("t", lobby.MaxTitleLength); rooms[0].Title != want {
		t.Fatalf("title length = %d, want %d", len(rooms[0].Title), lobby.MaxTitleLength)
	}
	if rooms[0].Description != rooms[0].Title {
		t.Fatalf("description = %q, want the title", rooms[0].Description)
	}

	longDescription := strings.Repeat("d", lobby.MaxDescriptionLength+10)
	app.do(t, http.MethodPost, "/rooms",
		`{"room_title":"Other","room_description":"`+longDescription+`"}`)

	stored, err := room.GetRoom(app.database, "other")
	if err != nil {
		t.Fatalf("GetRoom() error = %v", err)
	}
	if stored == nil || len(stored.Description) != lobby.MaxDescriptionLength {
		t.Fatalf("stored description = %q", stored.Description)
	}
}

func TestCreateRoomEmptyTitle(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.do(t, http.MethodPost, "/rooms", `{"room_title":"   "}`).
		AssertStatus(http.StatusNoContent)

	rooms, err := room.ListRooms(app.database)
	if err != nil {
		t.Fatalf("ListRooms() error = %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("rooms = %v, want none", rooms)
	}
}

func TestDeleteRoom(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	added, err := room.AddRoom(app.database, "Temp", "Gone soon")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	app.do(t, http.MethodDelete, "/rooms/"+added.ID, "").
		AssertStatus(http.StatusNoContent)

	stored, err := room.GetRoom(app.database, added.ID)
	if err != nil {
		t.Fatalf("GetRoom() error = %v", err)
	}
	if stored != nil {
		t.Fatalf("room still exists: %+v", stored)
	}
}

func TestDeleteMissingRoomPublishesNothing(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	events, err := app.relay.Subscribe(context.Background(), "*")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer events.Close()

	app.do(t, http.MethodDelete, "/rooms/missing", "").
		AssertStatus(http.StatusNoContent)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := events.Next(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Next() error = %v, want no event", err)
	}
}

func TestLiveInitialPatch(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	stream := openStream(t, app, signalURL("/live",
		`{"user_id":"abc12345","username":"HappyFox","color":"#000"}`))

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}
	if patch.Kind != stargotest.PatchElements ||
		patch.Selector != "#lobby-live" ||
		!strings.Contains(patch.Elements, "No rooms yet") {
		t.Fatalf("first patch = %+v", patch)
	}
}

func TestLivePatchesOnRoomChanges(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	stream := openStream(t, app, signalURL("/live",
		`{"user_id":"abc12345","username":"HappyFox","color":"#000"}`))

	if _, ok := stream.NextPatch(); !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}

	app.do(t, http.MethodPost, "/rooms",
		`{"room_title":"Design","room_description":"UI talk"}`).
		AssertStatus(http.StatusOK)

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no room patch")
	}
	if patch.Kind != stargotest.PatchElements ||
		patch.Selector != "#lobby-live" ||
		!strings.Contains(patch.Elements, "Design") {
		t.Fatalf("patch after change = %+v", patch)
	}
}

func TestLiveWithoutIdentityNavigates(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	stream := openStream(t, app, "/live")

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no navigation patch")
	}
	if !strings.Contains(patch.Elements, `window.location = "/"`) {
		t.Fatalf("missing navigation: %+v", patch)
	}
}
