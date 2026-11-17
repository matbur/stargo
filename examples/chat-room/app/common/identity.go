// Package common holds the chat-room page shell and demo visitor identity.
package common

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"uuid"

	"github.com/matbur/stargo/signals"
)

// SessionStorageKey is the browser key used to keep one identity per tab.
const SessionStorageKey = "chat_visitor"

var (
	adjectives = []string{
		"Happy", "Sleepy", "Grumpy", "Sneezy", "Bashful", "Dopey", "Doc",
		"Swift", "Clever", "Brave", "Gentle", "Mighty", "Sneaky", "Jolly",
		"Fuzzy",
	}
	animals = []string{
		"Panda", "Fox", "Owl", "Cat", "Dog", "Bear", "Wolf", "Tiger", "Lion",
		"Koala", "Bunny", "Penguin", "Otter", "Seal", "Duck", "Frog", "Sloth",
		"Deer", "Moose", "Falcon",
	}
	colors = []string{
		"#e74c3c", "#e67e22", "#f1c40f", "#2ecc71", "#1abc9c",
		"#3498db", "#9b59b6", "#e91e63", "#00bcd4", "#8bc34a",
	}
)

// Identity is one tab's demo visitor identity.
type Identity struct {
	UserID   string
	Username string
	Color    string
}

// Mint returns a fresh identity with an eight-character id.
func Mint() Identity {
	return Identity{
		UserID:   uuid.NewV4().String()[:8],
		Username: randomChoice(adjectives) + randomChoice(animals),
		Color:    randomChoice(colors),
	}
}

// ForPage returns the identity from the query string when all three values
// are present, otherwise a freshly minted one.
func ForPage(r *http.Request) Identity {
	if identity := FromQuery(r); identity != nil {
		return *identity
	}
	return Mint()
}

// FromQuery reads a complete identity override from the query string.
func FromQuery(r *http.Request) *Identity {
	query := r.URL.Query()
	identity := Identity{
		UserID:   query.Get("user_id"),
		Username: query.Get("username"),
		Color:    query.Get("color"),
	}
	if !identity.complete() {
		return nil
	}
	return &identity
}

// FromSignals reads the identity the browser sends with a Datastar request.
// An incomplete or absent identity returns nil without an error.
func FromSignals(r *http.Request) (*Identity, error) {
	payload, err := signals.Read(r)
	if err != nil {
		return nil, err
	}

	identity := Identity{
		UserID:   signalString(payload, "user_id"),
		Username: signalString(payload, "username"),
		Color:    signalString(payload, "color"),
	}
	if !identity.complete() {
		return nil, nil
	}
	return &identity, nil
}

// Signals returns the identity fields as Datastar signal values.
func (i Identity) Signals() map[string]any {
	return map[string]any{
		"user_id":  i.UserID,
		"username": i.Username,
		"color":    i.Color,
	}
}

func (i Identity) complete() bool {
	return i.UserID != "" && i.Username != "" && i.Color != ""
}

func signalString(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func randomChoice(values []string) string {
	return values[rand.IntN(len(values))]
}

// SessionInitScript restores the tab's identity from sessionStorage, or stores
// the page's identity there on first load.
func SessionInitScript() string {
	key, err := json.Marshal(SessionStorageKey)
	if err != nil {
		panic(err) // a constant string cannot fail to marshal
	}
	return fmt.Sprintf(`(() => {
  const key = %s;
  try {
    const raw = sessionStorage.getItem(key);
    if (raw) {
      const v = JSON.parse(raw);
      if (v.user_id) {
        $user_id = v.user_id;
        $username = v.username;
        $color = v.color;
      }
    } else if ($user_id) {
      sessionStorage.setItem(
        key,
        JSON.stringify({ user_id: $user_id, username: $username, color: $color }),
      );
    }
  } catch {}
})();`, key)
}
