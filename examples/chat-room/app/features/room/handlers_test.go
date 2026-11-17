package room_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
)

type handlerApp struct {
	app      *stargo.App
	database *chatdb.DB
	relay    *relay.Relay[struct{}]
}

func newHandlerApp(t *testing.T) handlerApp {
	t.Helper()

	database := testDB(t)
	hub := relay.New[struct{}]()
	app := stargo.New(stargo.WithTracer(nil))
	room.Page{Database: database, Relay: hub}.Register(app)
	t.Cleanup(func() { _ = app.Close() })

	return handlerApp{app: app, database: database, relay: hub}
}

func (a handlerApp) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, strings.NewReader(body))
	}

	recorder := httptest.NewRecorder()
	a.app.ServeHTTP(recorder, request)
	return recorder
}

func (a handlerApp) seedRoom(t *testing.T) room.Room {
	t.Helper()

	added, err := room.AddRoom(a.database, "Design Chat", "UI talk")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}
	return added
}

func (a handlerApp) seedUser(t *testing.T, roomID string) {
	t.Helper()

	if err := room.AddUser(a.database, room.User{
		ID: "u1", RoomID: roomID, Username: "HappyFox", Color: "#000",
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
}

func TestShowRoom(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	app.seedUser(t, added.ID)
	if err := room.AddMessage(app.database, room.Message{
		ID: "m1", RoomID: added.ID, UserID: "u1", Username: "HappyFox",
		Color: "#000", Text: "hello there", Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}

	recorder := app.do(t, http.MethodGet, "/rooms/"+added.ID, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"Design Chat",
		"UI talk",
		"hello there",
		"HappyFox",
		"1 online",
		`id="room-live"`,
		`/rooms/design-chat/live`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("room page missing %q", want)
		}
	}
}

func TestShowUnknownRoomRedirects(t *testing.T) {
	t.Parallel()

	recorder := newHandlerApp(t).do(t, http.MethodGet, "/rooms/nope", "")
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want /", got)
	}
}

func TestShowIdentityOverride(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)

	recorder := app.do(t, http.MethodGet,
		"/rooms/"+added.ID+"?user_id=abc12345&username=HappyFox&color=%23000", "")
	if body := recorder.Body.String(); !strings.Contains(body, "abc12345") {
		t.Fatalf("query identity missing: %s", body)
	}
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	app.seedUser(t, added.ID)
	if _, err := room.SetUserTyping(app.database, "u1", added.ID, true); err != nil {
		t.Fatalf("SetUserTyping() error = %v", err)
	}

	events, err := app.relay.Subscribe(context.Background(), room.MessageSubject(added.ID))
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer events.Close()

	recorder := app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"  hello  "}`)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}

	messages, err := room.ListMessages(app.database, added.ID, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	stored := messages[0]
	if stored.Text != "hello" || stored.UserID != "u1" || stored.Username != "HappyFox" {
		t.Fatalf("stored message = %+v", stored)
	}
	if stored.Timestamp <= 0 {
		t.Fatalf("timestamp = %d, want Unix milliseconds", stored.Timestamp)
	}

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].Typing {
		t.Fatalf("typing flag was not cleared: %+v", users)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if subject, _, err := events.Next(ctx); err != nil || subject != room.MessageSubject(added.ID) {
		t.Fatalf("relay Next() = %q, %v", subject, err)
	}
}

func TestSendMessageTruncates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	app.seedUser(t, added.ID)

	long := strings.Repeat("x", room.MaxMessageLength+50)
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"`+long+`"}`)

	messages, err := room.ListMessages(app.database, added.ID, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 1 || len(messages[0].Text) != room.MaxMessageLength {
		t.Fatalf("stored text length = %d, want %d", len(messages[0].Text), room.MaxMessageLength)
	}
}

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	app.seedUser(t, added.ID)

	recorder := app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"   "}`)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}

	messages, err := room.ListMessages(app.database, added.ID, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages = %v, want none", messages)
	}
}

func TestSendWithoutRoomOrPresenceNavigates(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)

	unknown := app.do(t, http.MethodPost, "/rooms/nope/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"hi"}`)
	if !strings.Contains(unknown.Body.String(), `window.location = "/"`) {
		t.Fatalf("unknown room response = %s", unknown.Body.String())
	}

	added := app.seedRoom(t)
	notPresent := app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"hi"}`)
	if !strings.Contains(notPresent.Body.String(), `window.location = "/rooms/design-chat"`) {
		t.Fatalf("absent user response = %s", notPresent.Body.String())
	}
}

func TestTypingPublishesOnlyChanges(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)
	app.seedUser(t, added.ID)

	events, err := app.relay.Subscribe(context.Background(), room.TypingSubject(added.ID))
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer events.Close()

	payload := `{"user_id":"u1","username":"HappyFox","color":"#000","message":"hi"}`
	if code := app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing", payload).Code; code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", code)
	}

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || !users[0].Typing {
		t.Fatalf("typing flag = %+v, want true", users)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if subject, _, err := events.Next(ctx); err != nil || subject != room.TypingSubject(added.ID) {
		t.Fatalf("relay Next() = %q, %v", subject, err)
	}

	// The same state again must not publish.
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing", payload)
	quiet, cancelQuiet := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelQuiet()
	if _, _, err := events.Next(quiet); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Next() error = %v, want no event", err)
	}

	// An empty message stops typing and publishes.
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"  "}`)
	users, err = room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].Typing {
		t.Fatalf("typing flag = %+v, want false", users)
	}
	stop, cancelStop := context.WithTimeout(context.Background(), time.Second)
	defer cancelStop()
	if _, _, err := events.Next(stop); err != nil {
		t.Fatalf("relay Next() after stop = %v", err)
	}
}

func TestTypingUnknownRoomOrUser(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	if code := app.do(t, http.MethodPost, "/rooms/nope/typing", `{"user_id":"u1"}`).Code; code != http.StatusNoContent {
		t.Fatalf("unknown room status = %d, want 204", code)
	}

	added := app.seedRoom(t)
	if code := app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing", `{"user_id":"u1"}`).Code; code != http.StatusNoContent {
		t.Fatalf("unknown user status = %d, want 204", code)
	}

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("users = %+v, want none", users)
	}
}
