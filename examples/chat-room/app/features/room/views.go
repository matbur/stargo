// Package room is the per-room chat page, its live stream, and its commands.
package room

import (
	"fmt"
	"strings"
	"time"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	ds "github.com/matbur/stargo/datastar"
	"github.com/matbur/stargo/examples/chat-room/app/common"
	"github.com/matbur/stargo/examples/chat-room/app/routes"
)

// LiveSelector identifies the element replaced by every live patch.
const LiveSelector = "#room-live"

const sendIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"/><path d="m21.854 2.147-10.94 10.939"/></svg>`

// LiveView is the patch target for SSE. It excludes the page shell that owns
// the live connection, so patches never restart it.
func LiveView(chatRoom Room, currentUserID string, messages []Message, users []User) g.Node {
	return h.Div(
		g.Attr("id", "room-live"),
		h.Div(
			g.Attr("class", "chat-header"),
			h.A(
				g.Attr("class", "back-link"),
				h.Href(routes.Lobby.Href()),
				g.Text("← Rooms"),
			),
			h.Div(
				g.Attr("class", "chat-title-wrap"),
				h.Div(g.Attr("class", "chat-title"), g.Text(chatRoom.Title)),
				h.Div(g.Attr("class", "chat-subtitle"), g.Text(chatRoom.Description)),
			),
			onlineUsersView(users),
		),
		h.Div(
			g.Attr("class", "chat-body"),
			messagesView(currentUserID, messages),
			typingIndicatorView(currentUserID, users),
		),
		h.Div(g.Attr("class", "chat-footer"), inputFormView(chatRoom.ID)),
	)
}

// View is the complete room page.
func View(identity common.Identity, chatRoom Room, messages []Message, users []User, liveHref string) g.Node {
	return common.Page(
		h.Div(
			g.Attr("id", "room"),
			g.Attr("class", "chat-container"),
			signalsAttribute(identity),
			ds.Init(liveInit(liveHref)),
			LiveView(chatRoom, identity.UserID, messages, users),
		),
	)
}

func liveInit(liveHref string) string {
	return common.SessionInitScript() + "\n" + ds.Get(liveHref, ds.WithRetry(ds.RetryAlways))
}

func signalsAttribute(identity common.Identity) g.Node {
	payload := identity.Signals()
	payload["message"] = ""
	return ds.Signals(payload, ds.WithIfMissing())
}

func messageView(message Message, currentUserID string) g.Node {
	class := "message"
	if message.UserID == currentUserID {
		class += " own"
	}

	return h.Div(
		g.Attr("data-msg-id", message.ID),
		g.Attr("class", class),
		h.Div(
			g.Attr("class", "message-header"),
			h.Span(
				g.Attr("class", "username"),
				g.Attr("style", "color: "+message.Color),
				g.Text(message.Username),
			),
			h.Span(
				g.Attr("class", "timestamp"),
				g.Text(time.UnixMilli(message.Timestamp).Format("15:04")),
			),
		),
		h.Div(g.Attr("class", "message-text"), g.Text(message.Text)),
	)
}

func messagesView(currentUserID string, messages []Message) g.Node {
	class := "messages"
	if len(messages) == 0 {
		class += " empty"
	}

	children := make([]g.Node, 0, len(messages)+1)
	if len(messages) == 0 {
		children = append(children, h.Div(
			g.Attr("class", "empty-state"),
			g.Text("No messages yet. Say hello!"),
		))
	} else {
		children = append(children, ds.On(
			"load",
			"setTimeout(() => el.scrollTop = el.scrollHeight, 10)",
		))
	}
	for _, message := range messages {
		children = append(children, messageView(message, currentUserID))
	}

	return h.Div(g.Attr("id", "messages"), g.Attr("class", class), g.Group(children))
}

func typingText(typingUsers []User) string {
	switch len(typingUsers) {
	case 0:
		return ""
	case 1:
		return typingUsers[0].Username + " is typing"
	case 2:
		return typingUsers[0].Username + " and " + typingUsers[1].Username + " are typing"
	default:
		return fmt.Sprintf("%s and %d others are typing", typingUsers[0].Username, len(typingUsers)-1)
	}
}

func typingIndicatorView(currentUserID string, users []User) g.Node {
	typingUsers := make([]User, 0, len(users))
	for _, user := range users {
		if user.Typing && user.ID != currentUserID {
			typingUsers = append(typingUsers, user)
		}
	}

	class := "typing-indicator"
	if len(typingUsers) == 0 {
		class += " hidden"
	}
	children := []g.Node{g.Attr("id", "typing"), g.Attr("class", class)}
	if text := typingText(typingUsers); text != "" {
		children = append(children,
			h.Span(g.Attr("class", "typing-text"), g.Text(text)),
			h.Span(
				g.Attr("class", "typing-dots"),
				h.Span(g.Attr("class", "dot"), g.Text(".")),
				h.Span(g.Attr("class", "dot"), g.Text(".")),
				h.Span(g.Attr("class", "dot"), g.Text(".")),
			),
		)
	}

	return h.Div(g.Group(children))
}

func onlineUsersView(users []User) g.Node {
	if len(users) == 0 {
		return h.Div(g.Attr("id", "online"), g.Attr("class", "online-users"))
	}

	children := []g.Node{
		g.Attr("id", "online"),
		g.Attr("class", "online-users"),
		h.Span(g.Attr("class", "online-label"), g.Text(fmt.Sprintf("%d online", len(users)))),
	}

	avatars := make([]g.Node, 0, 9)
	for _, user := range users[:min(len(users), 8)] {
		name := []rune(user.Username)
		initial := ""
		if len(name) > 0 {
			initial = strings.ToUpper(string(name[0]))
		}
		avatars = append(avatars, h.Span(
			g.Attr("class", "avatar"),
			g.Attr("title", user.Username),
			g.Attr("style", "background-color: "+user.Color),
			g.Text(initial),
		))
	}
	if len(users) > 8 {
		avatars = append(avatars, h.Span(
			g.Attr("class", "avatar more"),
			g.Text(fmt.Sprintf("+%d", len(users)-8)),
		))
	}
	children = append(children, h.Div(g.Attr("class", "avatars"), g.Group(avatars)))

	return h.Div(g.Group(children))
}

func inputFormView(roomID string) g.Node {
	send := ds.Post(routes.SendMessage.Href(roomID))
	typing := ds.Post(routes.SetTyping.Href(roomID))
	submit := "if ($message.trim()) { " + send + "; $message = ''; }"
	click := "if ($message.trim()) { " + send +
		"; $message = ''; document.getElementById('message-input').focus(); }"

	return h.Form(
		g.Attr("id", "input-form"),
		g.Attr("class", "input-form"),
		ds.On("submit", submit, ds.WithPrevent()),
		h.Input(
			g.Attr("id", "message-input"),
			g.Attr("type", "text"),
			g.Attr("class", "message-input"),
			g.Attr("placeholder", "Type a message..."),
			g.Attr("autocomplete", "off"),
			g.Attr("autofocus", ""),
			ds.Bind("message"),
			ds.On("input", typing),
		),
		h.Button(
			g.Attr("type", "button"),
			g.Attr("class", "send-button"),
			ds.Attr("disabled", "!$message"),
			ds.On("click", click),
			h.Span(g.Attr("class", "send-icon"), g.Raw(sendIcon)),
		),
	)
}
