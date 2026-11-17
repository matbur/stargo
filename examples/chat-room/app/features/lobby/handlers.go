package lobby

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/examples/chat-room/app/common"
	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
	"github.com/matbur/stargo/examples/chat-room/app/routes"
	"github.com/matbur/stargo/relay"
	"github.com/matbur/stargo/sse"
)

// Input limits applied before storing a room.
const (
	MaxTitleLength       = 60
	MaxDescriptionLength = 200
)

// Page registers the lobby routes and owns their dependencies.
type Page struct {
	Database *chatdb.DB
	Relay    *relay.Relay[struct{}]
}

// Register adds the lobby routes to app.
func (p Page) Register(app *stargo.App) {
	app.Handle(routes.Lobby, p.Show)
	app.Handle(routes.LobbyLive, p.Live)
	app.Handle(routes.CreateRoom, p.Create)
	app.Handle(routes.DeleteRoom, p.Delete)
}

// Show renders the lobby with a minted or query-supplied identity.
func (p Page) Show(c *stargo.Ctx) error {
	rooms, err := room.ListRooms(p.Database)
	if err != nil {
		return err
	}
	onlineCounts, err := room.CountUsersByRoom(p.Database)
	if err != nil {
		return err
	}

	return c.HTML(View(
		common.ForPage(c.R),
		rooms,
		onlineCounts,
		routes.LobbyLive.Href(),
	))
}

// Create adds a room from the create dialog signals and navigates to it.
func (p Page) Create(c *stargo.Ctx) error {
	input, err := ReadSignals(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "invalid signals")
	}

	title := common.Truncate(strings.TrimSpace(input.RoomTitle), MaxTitleLength)
	if title == "" {
		return c.NoContent()
	}

	description := strings.TrimSpace(input.RoomDescription)
	if description == "" {
		description = title
	}
	description = common.Truncate(description, MaxDescriptionLength)

	added, err := room.AddRoom(p.Database, title, description)
	if err != nil {
		return err
	}

	p.Relay.Publish(Changed, struct{}{})

	stream, err := c.Stream()
	if err != nil {
		return err
	}
	return stream.Navigate(routes.Room.Href(added.ID))
}

// Delete removes a room and notifies room watchers and the lobby.
func (p Page) Delete(c *stargo.Ctx) error {
	roomID := c.R.PathValue("room_id")

	deleted, err := room.DeleteRoom(p.Database, roomID)
	if err != nil {
		return err
	}
	if deleted {
		p.Relay.Publish(room.DeletedSubject(roomID), struct{}{})
		p.Relay.Publish(Changed, struct{}{})
	}

	return c.NoContent()
}

// Live streams lobby patches while rooms or presence change.
func (p Page) Live(c *stargo.Ctx) error {
	identity, err := common.FromSignals(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "invalid signals")
	}

	stream, err := c.Stream()
	if err != nil {
		return err
	}
	if identity == nil {
		// Reload the lobby page, which mints a fresh identity.
		return stream.Navigate(routes.Lobby.Href())
	}

	ctx := c.R.Context()
	subscription, err := p.Relay.Subscribe(ctx, "*")
	if err != nil {
		return err
	}
	defer subscription.Close()

	if err := p.patch(stream); err != nil {
		return nil // the client is already gone
	}

	for {
		subject, _, err := subscription.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, relay.ErrClosed) {
				return nil
			}
			return err
		}
		if !lobbySubject(subject) {
			continue
		}
		if err := p.patch(stream); err != nil {
			return nil // disconnected
		}
	}
}

func (p Page) patch(stream *sse.Stream) error {
	rooms, err := room.ListRooms(p.Database)
	if err != nil {
		return err
	}
	onlineCounts, err := room.CountUsersByRoom(p.Database)
	if err != nil {
		return err
	}
	return stream.PatchElements(
		LiveView(rooms, onlineCounts),
		sse.WithSelector(LiveSelector),
		sse.WithMode(sse.ModeOuter),
	)
}

// lobbySubject reports whether a relay subject changes the room list or a
// room's online count.
func lobbySubject(subject string) bool {
	return strings.HasPrefix(subject, "lobby.") ||
		strings.HasSuffix(subject, ".presence") ||
		strings.HasSuffix(subject, ".deleted")
}
