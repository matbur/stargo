package trace

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
	"uuid"
)

type spanContextKey struct{}

// ContextWithSpan attaches span to ctx. A nil span leaves the context
// unchanged.
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	if span == nil {
		return ctx
	}
	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext returns the active span, or nil when tracing is off.
func SpanFromContext(ctx context.Context) *Span {
	span, _ := ctx.Value(spanContextKey{}).(*Span)
	return span
}

// Context returns the span's W3C trace context.
func (s *Span) Context() SpanContext {
	if s == nil {
		return SpanContext{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return SpanContext{
		TraceID:    s.traceID,
		SpanID:     s.id,
		Sampled:    s.sampled,
		TraceState: s.traceState,
	}
}

// ExtractContext parses a W3C traceparent and tracestate pair. It reports
// false when the traceparent is absent or invalid.
func ExtractContext(traceparent, tracestate string) (SpanContext, bool) {
	parts := strings.Split(strings.TrimSpace(traceparent), "-")
	if len(parts) < 4 {
		return SpanContext{}, false
	}
	if !validHex(parts[0], 2) || strings.EqualFold(parts[0], "ff") {
		return SpanContext{}, false
	}
	if !validHex(parts[1], 32) || !validHex(parts[2], 16) || !validHex(parts[3], 2) {
		return SpanContext{}, false
	}

	traceBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return SpanContext{}, false
	}
	spanBytes, err := hex.DecodeString(parts[2])
	if err != nil {
		return SpanContext{}, false
	}
	flagBytes, err := hex.DecodeString(parts[3])
	if err != nil || len(flagBytes) != 1 {
		return SpanContext{}, false
	}

	var traceID uuid.UUID
	copy(traceID[:], traceBytes)
	if traceID == uuid.Nil() {
		return SpanContext{}, false
	}

	var spanID uuid.UUID
	copy(spanID[8:], spanBytes)
	if spanID == uuid.Nil() {
		return SpanContext{}, false
	}

	return SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		Sampled:    flagBytes[0]&1 == 1,
		TraceState: validTraceState(tracestate),
	}, true
}

// StartRemote opens a span that continues a remote trace. The remote span
// becomes the parent of the new span.
func (t *Tracer) StartRemote(name, kind string, remote SpanContext) *Span {
	if t == nil || len(t.sinks) == 0 || t.closed.Load() {
		return nil
	}
	if remote.TraceID == uuid.Nil() || remote.SpanID == uuid.Nil() {
		return t.start(name, kind)
	}
	sampled := t.sample(remote.TraceID, remote, true)
	return t.newSpan(uuid.NewV7(), remote.TraceID, remote.SpanID, name, kind, sampled, remote.TraceState)
}

// Inject writes the active span's traceparent and tracestate into header. It
// writes nothing when no span is active.
func Inject(ctx context.Context, header http.Header) {
	if header == nil {
		return
	}
	span := SpanFromContext(ctx)
	if span == nil {
		return
	}
	spanContext := span.Context()
	if spanContext.TraceID == uuid.Nil() || spanContext.SpanID == uuid.Nil() {
		return
	}
	header.Set("traceparent", formatTraceparent(spanContext))
	if spanContext.TraceState != "" {
		header.Set("tracestate", spanContext.TraceState)
	}
}

func formatTraceparent(spanContext SpanContext) string {
	flags := "00"
	if spanContext.Sampled {
		flags = "01"
	}
	return "00-" + hex.EncodeToString(spanContext.TraceID[:]) +
		"-" + hex.EncodeToString(spanContext.SpanID[8:]) +
		"-" + flags
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validTraceState(tracestate string) string {
	tracestate = strings.TrimSpace(tracestate)
	if tracestate == "" || len(tracestate) > 512 {
		return ""
	}
	if strings.ContainsAny(tracestate, "\r\n") {
		return ""
	}
	return tracestate
}
