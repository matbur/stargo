package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"uuid"
)

const maxEvents = 1000

// Span is one timed operation in a trace tree.
//
// A nil *Span is valid and every method on it is a no-op, so handlers need no
// tracing conditionals.
type Span struct {
	mu       sync.Mutex
	tracer   *Tracer
	id       uuid.UUID
	traceID  uuid.UUID
	parentID uuid.UUID
	name     string
	kind     string
	startNS  int64
	endNS    int64
	status   string
	err      string
	attrs    map[string]any
	events   []event
	dropped  uint64
	ended    bool
}

type event struct {
	TimeNS     int64          `json:"time_ns"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type record struct {
	SpanID     string         `json:"span_id"`
	TraceID    string         `json:"trace_id"`
	ParentID   string         `json:"parent_id,omitempty"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	StartNS    int64          `json:"start_ns"`
	EndNS      int64          `json:"end_ns"`
	DurationNS int64          `json:"duration_ns"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Events     []event        `json:"events,omitempty"`
}

// ID returns the span id.
func (s *Span) ID() uuid.UUID {
	if s == nil {
		return uuid.Nil()
	}
	return s.id
}

// TraceID returns the id shared by all spans in this trace.
func (s *Span) TraceID() uuid.UUID {
	if s == nil {
		return uuid.Nil()
	}
	return s.traceID
}

// Attr sets an attribute. Attributes set after End are ignored.
func (s *Span) Attr(key string, value any) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.attrs == nil {
		s.attrs = make(map[string]any, 4)
	}
	s.attrs[key] = value
}

// Event records a timestamped event with optional attributes. At most 1000
// events are kept per span; the rest are counted in span.dropped_events.
func (s *Span) Event(name string, attributes map[string]any) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if len(s.events) >= maxEvents {
		s.dropped++
		return
	}
	s.events = append(s.events, event{
		TimeNS:     time.Now().UnixNano(),
		Name:       name,
		Attributes: attributes,
	})
}

// SetKind changes the span kind while the span is open. The framework uses it
// to mark long-lived SSE handlers as streams.
func (s *Span) SetKind(kind string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.kind = kind
}

// Fail marks the span as failed and records the error message.
func (s *Span) Fail(err error) {
	if s == nil || err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.status = "error"
	s.err = err.Error()
}

// Step opens a child span. The child shares the trace id and names this span
// as its parent.
//
// A step on an already ended span returns an error instead of creating an
// orphan. On a nil span it returns nil without an error, because tracing is
// disabled.
func (s *Span) Step(name string) (*Span, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	ended := s.ended
	tracer := s.tracer
	traceID := s.traceID
	parentID := s.id
	kind := s.kind
	s.mu.Unlock()
	if ended {
		return nil, fmt.Errorf("trace: cannot step from ended span %q", s.name)
	}
	if tracer == nil {
		return nil, nil
	}
	return tracer.startChild(name, kind, traceID, parentID), nil
}

// Do runs fn inside a child span and ends the child afterwards.
//
// A panic is recorded on the child and propagated to the caller, so the
// application's panic recovery still runs.
func (s *Span) Do(ctx context.Context, name string, fn func(context.Context) error) (err error) {
	child, err := s.Step(name)
	if err != nil {
		return err
	}
	if child == nil {
		return fn(ctx)
	}
	defer child.End()
	defer func() {
		if recovered := recover(); recovered != nil {
			child.Fail(fmt.Errorf("panic: %v", recovered))
			child.Event("panic", map[string]any{"value": fmt.Sprint(recovered)})
			panic(recovered)
		}
	}()
	return fn(ctx)
}

// End closes the span and enqueues it for export. Calling End more than once
// is harmless; a span never ended is never exported.
func (s *Span) End() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	s.endNS = time.Now().UnixNano()
	if s.dropped > 0 {
		if s.attrs == nil {
			s.attrs = make(map[string]any, 1)
		}
		s.attrs["span.dropped_events"] = s.dropped
	}
	rec := s.recordLocked()
	tracer := s.tracer
	s.mu.Unlock()

	if tracer == nil || tracer.writer == nil {
		return
	}
	line, err := json.Marshal(rec)
	if err != nil {
		tracer.writer.dropped.Add(1)
		return
	}
	tracer.writer.enqueue(line)
}

func (s *Span) recordLocked() record {
	rec := record{
		SpanID:     s.id.String(),
		TraceID:    s.traceID.String(),
		Name:       s.name,
		Kind:       s.kind,
		StartNS:    s.startNS,
		EndNS:      s.endNS,
		DurationNS: s.endNS - s.startNS,
		Status:     s.status,
		Error:      s.err,
		Attributes: s.attrs,
	}
	if s.parentID != uuid.Nil() {
		rec.ParentID = s.parentID.String()
	}
	if len(s.events) > 0 {
		rec.Events = s.events
	}
	return rec
}

func (t *Tracer) start(name, kind string) *Span {
	id := uuid.NewV7()
	return &Span{
		tracer:  t,
		id:      id,
		traceID: id,
		name:    name,
		kind:    kind,
		startNS: time.Now().UnixNano(),
		status:  "ok",
	}
}

func (t *Tracer) startChild(name, kind string, traceID, parentID uuid.UUID) *Span {
	span := &Span{
		tracer:   t,
		id:       uuid.NewV7(),
		traceID:  traceID,
		parentID: parentID,
		name:     name,
		kind:     kind,
		startNS:  time.Now().UnixNano(),
		status:   "ok",
	}
	return span
}
