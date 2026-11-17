package lobby

import (
	"net/http"
	"strings"

	"github.com/matbur/stargo/signals"
)

// Signals is the lobby page's signal shape.
type Signals struct {
	RoomTitle       string
	RoomDescription string
}

// ReadSignals reads the create-room dialog fields from a Datastar request.
func ReadSignals(r *http.Request) (Signals, error) {
	payload, err := signals.Read(r)
	if err != nil {
		return Signals{}, err
	}
	return Signals{
		RoomTitle:       signalString(payload, "room_title"),
		RoomDescription: signalString(payload, "room_description"),
	}, nil
}

func signalString(payload map[string]any, key string) string {
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
