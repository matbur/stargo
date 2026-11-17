package lobby_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/lobby"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
)

type testApp struct {
	app      *stargo.App
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

	return testApp{app: app, database: database, relay: hub}
}

func (a testApp) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var request *http.Request
	if reader != nil {
		request = httptest.NewRequest(method, target, reader)
	} else {
		request = httptest.NewRequest(method, target, nil)
	}

	recorder := httptest.NewRecorder()
	a.app.ServeHTTP(recorder, request)
	return recorder
}

func signalURL(path, payload string) string {
	return path + "?datastar=" + url.QueryEscape(payload)
}

func TestShowEmptyLobby(t *testing.T) {
	t.Parallel()

	recorder := newTestApp(t).do(t, http.MethodGet, "/", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"No rooms yet",
		`id="lobby-live"`,
		"data-signals__ifmissing",
		`@get(&#34;/live&#34;`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("lobby missing %q", want)
		}
	}
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

	body := app.do(t, http.MethodGet, "/", "").Body.String()
	for _, want := range []string{"Design", "UI talk", "1 online", `href="/rooms/design"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("lobby missing %q", want)
		}
	}
}

func TestShowMintsDistinctIdentities(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	first := app.do(t, http.MethodGet, "/", "").Body.String()
	second := app.do(t, http.MethodGet, "/", "").Body.String()
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

	recorder := app.do(t, http.MethodPost, "/rooms",
		`{"room_title":"Design Chat","room_description":"UI talk"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"/rooms/design-chat"`) {
		t.Fatalf("response missing navigation: %s", body)
	}

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
	recorder := app.do(t, http.MethodPost, "/rooms", `{"room_title":"   "}`)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}

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

	if code := app.do(t, http.MethodDelete, "/rooms/"+added.ID, "").Code; code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", code)
	}

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

	if code := app.do(t, http.MethodDelete, "/rooms/missing", "").Code; code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := events.Next(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Next() error = %v, want no event", err)
	}
}

func TestLiveInitialPatch(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		signalURL(server.URL+"/live",
			`{"user_id":"abc12345","username":"HappyFox","color":"#000"}`), nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	event := nextEvent(t, reader)
	if !strings.Contains(event, "datastar-patch-elements") ||
		!strings.Contains(event, "selector #lobby-live") {
		t.Fatalf("first event = %s", event)
	}
	if !strings.Contains(event, "No rooms yet") {
		t.Fatalf("first patch missing the empty state: %s", event)
	}
}

func TestLivePatchesOnRoomChanges(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		signalURL(server.URL+"/live",
			`{"user_id":"abc12345","username":"HappyFox","color":"#000"}`), nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	_ = nextEvent(t, reader) // initial patch

	created, err := http.Post(server.URL+"/rooms", "application/json",
		strings.NewReader(`{"room_title":"Design","room_description":"UI talk"}`))
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	defer created.Body.Close()
	if created.StatusCode != http.StatusOK {
		t.Fatalf("create room status = %d, want 200", created.StatusCode)
	}

	event := nextEvent(t, reader)
	if !strings.Contains(event, "Design") || !strings.Contains(event, "selector #lobby-live") {
		t.Fatalf("patch after change = %s", event)
	}
}

func TestLiveWithoutIdentityNavigates(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	server := httptest.NewServer(app.app)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/live", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()

	event := nextEvent(t, bufio.NewReader(response.Body))
	if !strings.Contains(event, `window.location = "/"`) {
		t.Fatalf("missing navigation: %s", event)
	}
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
