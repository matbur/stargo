// Package signals reads Datastar request signals.
package signals

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// DefaultMaxSize is the default payload limit: 1 MiB.
const DefaultMaxSize int64 = 1 << 20

// ErrTooLarge is returned when the payload exceeds the configured limit.
var ErrTooLarge = errors.New("signals: payload too large")

// ErrInvalid is returned when the payload is not a JSON object.
var ErrInvalid = errors.New("signals: payload is not a JSON object")

// Option configures Read.
type Option func(*config)

type config struct {
	maxSize int64
}

// MaxSize overrides the payload limit in bytes.
func MaxSize(bytes int64) Option {
	return func(c *config) {
		if bytes > 0 {
			c.maxSize = bytes
		}
	}
}

// Read decodes the signals sent with a request.
//
// GET, HEAD, and DELETE read the `datastar` query parameter; other methods
// read the request body. An absent payload returns an empty map. Values are
// returned as decoded JSON and are untrusted: callers validate types and
// shapes before using them.
func Read(request *http.Request, opts ...Option) (map[string]any, error) {
	config := config{maxSize: DefaultMaxSize}
	for _, opt := range opts {
		opt(&config)
	}

	var (
		raw []byte
		err error
	)

	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		value := request.URL.Query().Get("datastar")
		if int64(len(value)) > config.maxSize {
			return nil, ErrTooLarge
		}
		raw = []byte(value)
	default:
		if request.Body == nil {
			return map[string]any{}, nil
		}
		// Read one byte past the limit so an oversized payload is detected
		// without buffering it whole.
		raw, err = io.ReadAll(io.LimitReader(request.Body, config.maxSize+1))
		if err != nil {
			return nil, err
		}
		if int64(len(raw)) > config.maxSize {
			return nil, ErrTooLarge
		}
	}

	if len(raw) == 0 {
		return map[string]any{}, nil
	}

	var signals map[string]any
	if err := json.Unmarshal(raw, &signals); err != nil {
		return nil, ErrInvalid
	}
	if signals == nil {
		return nil, ErrInvalid
	}
	return signals, nil
}
