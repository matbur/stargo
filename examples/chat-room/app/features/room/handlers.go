package room

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/examples/chat-room/app/common"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/routes"
	"github.com/matbur/stargo/relay"
	"github.com/matbur/stargo/sse"
)

// MaxMessageLength is the number of runes kept from one message.
const MaxMessageLength = 2000

// Page registers the room routes and owns their dependencies.
type Page struct {
	Database *chatdb.DB
	Relay    *relay.Relay[struct{}]
}

// Register adds the room routes to app.
func (p Page) Register(app *stargo.App) {
	app.Handle(routes.Room, p.Show)
	app.Handle(routes.RoomLive, p.Live)
	app.Handle(routes.SendMessage, p.Send)
	app.Handle(routes.SetTyping, p.Typing)
}

// Show renders one room with its message history and present users.
func (p Page) Show(c *stargo.Ctx) error {
	chatRoom, err := p.room(c)
	if err != nil {
		return err
	}
	if chatRoom == nil {
		return c.Redirect(http.StatusFound, routes.Lobby.Href())
	}

	messages, err := ListMessages(p.Database, chatRoom.ID, 0)
	if err != nil {
		return err
	}
	users, err := ListUsers(p.Database, chatRoom.ID)
	if err != nil {
		return err
	}

	return c.HTML(View(
		common.ForPage(c.R),
		*chatRoom,
		messages,
		users,
		routes.RoomLive.Href(chatRoom.ID),
	))
}

// Send stores one message from a present user and publishes a room event.
func (p Page) Send(c *stargo.Ctx) error {
	chatRoom, err := p.room(c)
	if err != nil {
		return err
	}
	if chatRoom == nil {
		return navigate(c, routes.Lobby.Href())
	}

	input, err := ReadSignals(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "invalid signals")
	}
	if input.UserID == "" {
		return navigate(c, routes.Room.Href(chatRoom.ID))
	}
	exists, err := UserExists(p.Database, input.UserID, chatRoom.ID)
	if err != nil {
		return err
	}
	if !exists {
		return navigate(c, routes.Room.Href(chatRoom.ID))
	}

	text := common.Truncate(strings.TrimSpace(input.Message), MaxMessageLength)
	if text == "" {
		return c.NoContent()
	}

	message := Message{
		ID:        uuid.NewV4().String()[:8],
		RoomID:    chatRoom.ID,
		UserID:    input.UserID,
		Username:  input.Username,
		Color:     input.Color,
		Text:      text,
		Timestamp: time.Now().UnixMilli(),
	}
	if err := AddMessage(p.Database, message); err != nil {
		return err
	}
	if _, err := SetUserTyping(p.Database, input.UserID, chatRoom.ID, false); err != nil {
		return err
	}

	p.Relay.Publish(MessageSubject(chatRoom.ID), struct{}{})
	return c.NoContent()
}

// Typing stores the sender's typing flag and publishes only real changes.
func (p Page) Typing(c *stargo.Ctx) error {
	chatRoom, err := p.room(c)
	if err != nil {
		return err
	}
	if chatRoom == nil {
		return c.NoContent()
	}

	input, err := ReadSignals(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "invalid signals")
	}
	if input.UserID == "" {
		return c.NoContent()
	}
	exists, err := UserExists(p.Database, input.UserID, chatRoom.ID)
	if err != nil {
		return err
	}
	if !exists {
		return c.NoContent()
	}

	changed, err := SetUserTyping(
		p.Database,
		input.UserID,
		chatRoom.ID,
		strings.TrimSpace(input.Message) != "",
	)
	if err != nil {
		return err
	}
	if changed {
		p.Relay.Publish(TypingSubject(chatRoom.ID), struct{}{})
	}

	return c.NoContent()
}

// Live streams room patches, registers presence, and cleans it up on exit.
func (p Page) Live(c *stargo.Ctx) error {
	chatRoom, err := p.room(c)
	if err != nil {
		return err
	}
	if chatRoom == nil {
		return navigate(c, routes.Lobby.Href())
	}

	input, err := ReadSignals(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "invalid signals")
	}

	stream, err := c.Stream()
	if err != nil {
		return err
	}
	if input.UserID == "" {
		// Reload the room page, which mints a fresh identity.
		return stream.Navigate(routes.Room.Href(chatRoom.ID))
	}

	ctx := c.R.Context()
	subscription, err := p.Relay.Subscribe(ctx, Prefix(chatRoom.ID))
	if err != nil {
		return err
	}
	defer subscription.Close()

	user := User{
		ID:       input.UserID,
		RoomID:   chatRoom.ID,
		Username: input.Username,
		Color:    input.Color,
	}
	if err := AddUser(p.Database, user); err != nil {
		return err
	}
	defer func() {
		_ = RemoveUser(p.Database, user.ID, chatRoom.ID)
		p.Relay.Publish(PresenceSubject(chatRoom.ID), struct{}{})
	}()
	p.Relay.Publish(PresenceSubject(chatRoom.ID), struct{}{})

	if err := p.patch(stream, *chatRoom, input.UserID); err != nil {
		return nil // the client is already gone
	}

	for {
		_, _, err := subscription.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, relay.ErrClosed) {
				return nil
			}
			return err
		}

		current, err := GetRoom(p.Database, chatRoom.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return stream.Navigate(routes.Lobby.Href())
		}

		if err := p.patch(stream, *current, input.UserID); err != nil {
			return nil // disconnected
		}
	}
}

func (p Page) patch(stream *sse.Stream, chatRoom Room, currentUserID string) error {
	messages, err := ListMessages(p.Database, chatRoom.ID, 0)
	if err != nil {
		return err
	}
	users, err := ListUsers(p.Database, chatRoom.ID)
	if err != nil {
		return err
	}
	return stream.PatchElements(
		LiveView(chatRoom, currentUserID, messages, users),
		sse.WithSelector(LiveSelector),
		sse.WithMode(sse.ModeOuter),
	)
}

func (p Page) room(c *stargo.Ctx) (*Room, error) {
	roomID := c.R.PathValue("room_id")
	if roomID == "" {
		return nil, nil
	}
	return GetRoom(p.Database, roomID)
}

func navigate(c *stargo.Ctx, href string) error {
	stream, err := c.Stream()
	if err != nil {
		return err
	}
	return stream.Navigate(href)
}
