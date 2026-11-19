package room_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/relay"
	stargotest "github.com/matbur/stargo/testing"
)

type handlerApp struct {
	app      *stargo.App
	client   *stargotest.Client
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

	return handlerApp{
		app:      app,
		client:   stargotest.NewClient(t, app, stargotest.WithoutRedirects()),
		database: database,
		relay:    hub,
	}
}

func (a handlerApp) do(t *testing.T, method, target, body string) *stargotest.Response {
	t.Helper()

	var opts []stargotest.RequestOption
	if body != "" {
		opts = append(opts, stargotest.WithBody("application/json", []byte(body)))
	}
	return a.client.Do(a.client.Request(method, target, opts...))
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

	app.do(t, http.MethodGet, "/rooms/"+added.ID, "").
		AssertStatus(http.StatusOK).
		AssertContains(
			"Design Chat",
			"UI talk",
			"hello there",
			"HappyFox",
			"1 online",
			`id="room-live"`,
			`/rooms/design-chat/live`,
		)
}

func TestShowUnknownRoomRedirects(t *testing.T) {
	t.Parallel()

	newHandlerApp(t).do(t, http.MethodGet, "/rooms/nope", "").
		AssertStatus(http.StatusFound).
		AssertHeader("Location", "/")
}

func TestShowIdentityOverride(t *testing.T) {
	t.Parallel()

	app := newHandlerApp(t)
	added := app.seedRoom(t)

	app.do(t, http.MethodGet,
		"/rooms/"+added.ID+"?user_id=abc12345&username=HappyFox&color=%23000", "").
		AssertContains("abc12345")
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

	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"  hello  "}`).
		AssertStatus(http.StatusNoContent)

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

	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"   "}`).
		AssertStatus(http.StatusNoContent)

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

	app.do(t, http.MethodPost, "/rooms/nope/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"hi"}`).
		AssertContains(`window.location = "/"`)

	added := app.seedRoom(t)
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/send",
		`{"user_id":"u1","username":"HappyFox","color":"#000","message":"hi"}`).
		AssertContains(`window.location = "/rooms/design-chat"`)
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
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing", payload).
		AssertStatus(http.StatusNoContent)

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
	app.do(t, http.MethodPost, "/rooms/nope/typing", `{"user_id":"u1"}`).
		AssertStatus(http.StatusNoContent)

	added := app.seedRoom(t)
	app.do(t, http.MethodPost, "/rooms/"+added.ID+"/typing", `{"user_id":"u1"}`).
		AssertStatus(http.StatusNoContent)

	users, err := room.ListUsers(app.database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("users = %+v, want none", users)
	}
}
