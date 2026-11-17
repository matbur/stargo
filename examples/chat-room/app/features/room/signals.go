package room

import (
	"net/http"
	"strings"

	"github.com/matbur/stargo/signals"
)

// Signals is one command request's signal shape.
type Signals struct {
	UserID   string
	Username string
	Color    string
	Message  string
}

// ReadSignals reads the identity and message fields from a Datastar request.
func ReadSignals(r *http.Request) (Signals, error) {
	payload, err := signals.Read(r)
	if err != nil {
		return Signals{}, err
	}
	return Signals{
		UserID:   signalString(payload, "user_id"),
		Username: signalString(payload, "username"),
		Color:    signalString(payload, "color"),
		Message:  signalString(payload, "message"),
	}, nil
}

func signalString(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
