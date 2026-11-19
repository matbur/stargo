package stargotest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotPatch reports that an event is not a Datastar patch event.
var ErrNotPatch = errors.New("stargotest: event is not a Datastar patch")

// PatchKind identifies the kind of a Datastar patch.
type PatchKind int

// Datastar patch kinds.
const (
	PatchElements PatchKind = iota + 1
	PatchSignals
)

const (
	eventPatchElements = "datastar-patch-elements"
	eventPatchSignals  = "datastar-patch-signals"
)

// String returns the event name for the patch kind.
func (k PatchKind) String() string {
	switch k {
	case PatchElements:
		return eventPatchElements
	case PatchSignals:
		return eventPatchSignals
	default:
		return "unknown"
	}
}

// Event is one parsed Server-Sent Event.
type Event struct {
	// Type is the event type from the event field.
	Type string
	// ID is the event id.
	ID string
	// Retry is the reconnection interval from the retry field. It is zero
	// when the event does not carry one.
	Retry time.Duration
	// Data holds the data lines joined with a newline.
	Data string
	// Comments holds comments in the event block, in order.
	Comments []string
	// Raw is the raw event block without the terminating blank line.
	Raw string
}

// Patch is a parsed Datastar element or signal patch.
type Patch struct {
	// Kind is the patch kind.
	Kind PatchKind
	// Mode is the element patch mode.
	Mode string
	// Selector is the element patch selector.
	Selector string
	// Namespace is the element patch namespace.
	Namespace string
	// UseViewTransition reports whether the patch uses a view transition.
	UseViewTransition bool
	// ViewTransitionSelector is the view transition selector.
	ViewTransitionSelector string
	// Elements is the HTML payload, with element lines joined by a newline.
	Elements string
	// Signals holds the decoded signal values.
	Signals map[string]any
	// OnlyIfMissing reports whether signals are patched only when absent.
	OnlyIfMissing bool
}

// Patch parses the event as a Datastar patch. It returns ErrNotPatch for
// events of any other type.
func (e Event) Patch() (Patch, error) {
	switch e.Type {
	case eventPatchElements:
		return parseElementPatch(e)
	case eventPatchSignals:
		return parseSignalPatch(e)
	default:
		return Patch{}, ErrNotPatch
	}
}

func parseElementPatch(event Event) (Patch, error) {
	patch := Patch{Kind: PatchElements}
	var elements []string

	for _, line := range strings.Split(event.Data, "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "mode":
			patch.Mode = value
		case "selector":
			patch.Selector = value
		case "namespace":
			patch.Namespace = value
		case "useViewTransition":
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return Patch{}, fmt.Errorf("invalid useViewTransition %q: %w", value, err)
			}
			patch.UseViewTransition = enabled
		case "viewTransitionSelector":
			patch.ViewTransitionSelector = value
		case "elements":
			elements = append(elements, value)
		}
	}
	patch.Elements = strings.Join(elements, "\n")
	return patch, nil
}

func parseSignalPatch(event Event) (Patch, error) {
	patch := Patch{Kind: PatchSignals, Signals: map[string]any{}}
	var payload []string

	for _, line := range strings.Split(event.Data, "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "onlyIfMissing":
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return Patch{}, fmt.Errorf("invalid onlyIfMissing %q: %w", value, err)
			}
			patch.OnlyIfMissing = enabled
		case "signals":
			payload = append(payload, value)
		}
	}

	raw := strings.Join(payload, "\n")
	if raw == "" {
		return patch, nil
	}
	if err := json.Unmarshal([]byte(raw), &patch.Signals); err != nil {
		return Patch{}, fmt.Errorf("invalid signal JSON: %w", err)
	}
	return patch, nil
}

// Stream is a streaming HTTP response from a test client.
type Stream struct {
	// StatusCode is the response status code.
	StatusCode int
	// Header holds the response headers.
	Header http.Header
	// Request is the request that opened the stream.
	Request *http.Request

	tb       TB
	response *http.Response
	reader   *bufio.Reader

	closed    bool
	closeOnce sync.Once
	closeErr  error
}

// Open sends request and returns a streaming response. Status and headers are
// available before the first event is read.
func (c *Client) Open(request *http.Request) *Stream {
	c.tb.Helper()

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.tb.Fatalf("stargotest: %s %s: %v", request.Method, request.URL, err)
		return nil
	}
	return &Stream{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Request:    request,
		tb:         c.tb,
		response:   response,
		reader:     bufio.NewReader(response.Body),
	}
}

// Close closes the response body. Closing an already closed stream is
// harmless.
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		s.closed = true
		s.closeErr = s.response.Body.Close()
	})
	return s.closeErr
}

// Next returns the next event. It reports false when the stream ends or has
// been closed.
func (s *Stream) Next() (Event, bool) {
	s.tb.Helper()

	if s.closed {
		return Event{}, false
	}
	raw, fields, ok := s.readBlock()
	if !ok {
		return Event{}, false
	}
	event, err := parseEvent(raw, fields)
	if err != nil {
		s.tb.Fatalf("stargotest: parse SSE event: %v\nevent:\n%s", err, strings.Join(raw, "\n"))
		return Event{}, false
	}
	return event, true
}

// NextPatch returns the next Datastar patch, skipping events that are not
// patches. It reports false when the stream ends.
func (s *Stream) NextPatch() (Patch, bool) {
	s.tb.Helper()

	for {
		event, ok := s.Next()
		if !ok {
			return Patch{}, false
		}
		patch, err := event.Patch()
		if err == nil {
			return patch, true
		}
		if errors.Is(err, ErrNotPatch) {
			continue
		}
		s.tb.Fatalf("stargotest: parse SSE patch: %v\nevent:\n%s", err, event.Raw)
		return Patch{}, false
	}
}

type eventField struct {
	name    string
	value   string
	comment bool
}

// readBlock reads one event block. A block ends at a blank line or at end of
// stream. Leading blank lines are skipped.
func (s *Stream) readBlock() ([]string, []eventField, bool) {
	var raw []string
	var fields []eventField

	for {
		line, err := s.reader.ReadString('\n')
		if line == "" && err != nil {
			if len(raw) > 0 {
				return raw, fields, true
			}
			return nil, nil, false
		}

		line = trimLineEnding(line)
		if line == "" {
			if len(raw) > 0 {
				return raw, fields, true
			}
			if err != nil {
				return nil, nil, false
			}
			continue
		}

		raw = append(raw, line)
		fields = append(fields, parseEventField(line))
		if err != nil {
			if err == io.EOF {
				return raw, fields, true
			}
			s.tb.Fatalf("stargotest: read SSE stream: %v", err)
			return nil, nil, false
		}
	}
}

func trimLineEnding(line string) string {
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r")
}

func parseEventField(line string) eventField {
	if strings.HasPrefix(line, ":") {
		comment := strings.TrimPrefix(line, ":")
		comment = strings.TrimPrefix(comment, " ")
		return eventField{value: comment, comment: true}
	}

	name, value, found := strings.Cut(line, ":")
	if !found {
		return eventField{name: line}
	}
	return eventField{name: name, value: strings.TrimPrefix(value, " ")}
}

func parseEvent(raw []string, fields []eventField) (Event, error) {
	event := Event{Raw: strings.Join(raw, "\n")}
	var data []string

	for _, field := range fields {
		if field.comment {
			event.Comments = append(event.Comments, field.value)
			continue
		}
		switch field.name {
		case "event":
			event.Type = field.value
		case "id":
			event.ID = field.value
		case "data":
			data = append(data, field.value)
		case "retry":
			milliseconds, err := strconv.Atoi(field.value)
			if err != nil || milliseconds < 0 {
				return Event{}, fmt.Errorf("invalid retry %q", field.value)
			}
			event.Retry = time.Duration(milliseconds) * time.Millisecond
		}
	}

	event.Data = strings.Join(data, "\n")
	return event, nil
}
