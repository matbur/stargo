package trace

import (
	"context"
	"net/http"
	"testing"
	"uuid"
)

const (
	testTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testTraceState  = "vendor=value"
)

func TestExtractContext(t *testing.T) {
	t.Parallel()

	spanContext, ok := ExtractContext(testTraceparent, testTraceState)
	if !ok {
		t.Fatal("valid traceparent was rejected")
	}
	wantTrace := uuid.MustParse("4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
	if spanContext.TraceID != wantTrace {
		t.Fatalf("trace id = %s", spanContext.TraceID)
	}
	wantSpan := uuid.MustParse("00000000-0000-0000-00f0-67aa0ba902b7")
	if spanContext.SpanID != wantSpan {
		t.Fatalf("span id = %s, want %s", spanContext.SpanID, wantSpan)
	}
	if !spanContext.Sampled {
		t.Fatal("sampled flag was not read")
	}
	if spanContext.TraceState != testTraceState {
		t.Fatalf("trace state = %q", spanContext.TraceState)
	}

	notSampled, ok := ExtractContext("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00", "")
	if !ok || notSampled.Sampled {
		t.Fatalf("flags 00 = %+v, %v", notSampled, ok)
	}
}

func TestExtractContextRejectsInvalid(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing":       "",
		"short":         "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
		"bad version":   "zz-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"forbidden":     "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"short trace":   "00-4bf92f3577b34da6a3ce929d0e0e47-00f067aa0ba902b7-01",
		"short span":    "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba9-01",
		"bad flags":     "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-0z",
		"zero trace":    "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"zero span":     "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"not hex trace": "00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01",
	}
	for name, value := range cases {
		if _, ok := ExtractContext(value, ""); ok {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestExtractContextAcceptsFutureVersion(t *testing.T) {
	t.Parallel()

	spanContext, ok := ExtractContext(
		"cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
		"",
	)
	if !ok {
		t.Fatal("future version was rejected")
	}
	if !spanContext.Sampled {
		t.Fatal("future version lost the sampled flag")
	}
}

func TestInjectRoundTrip(t *testing.T) {
	t.Parallel()

	buffer := &lockedBuffer{}
	tracer := New(Destination(buffer), BatchSize(1), FlushInterval(0))
	t.Cleanup(func() { _ = tracer.Close() })

	span := tracer.Start("request", "request")
	ctx := ContextWithSpan(context.Background(), span)
	if SpanFromContext(ctx) != span {
		t.Fatal("SpanFromContext() did not return the active span")
	}

	header := http.Header{}
	Inject(ctx, header)
	extracted, ok := ExtractContext(header.Get("traceparent"), header.Get("tracestate"))
	if !ok {
		t.Fatalf("injected traceparent is invalid: %q", header.Get("traceparent"))
	}
	spanID := span.ID()
	var expectedSpan uuid.UUID
	copy(expectedSpan[8:], spanID[8:])
	if extracted.TraceID != span.TraceID() || extracted.SpanID != expectedSpan {
		t.Fatalf("round trip = %+v, span = %s/%s", extracted, span.TraceID(), spanID)
	}
	if !extracted.Sampled {
		t.Fatal("sampled span lost its flag")
	}
}

func TestInjectWithoutSpan(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	Inject(context.Background(), header)
	if len(header) != 0 {
		t.Fatalf("headers = %v, want none", header)
	}
	if SpanFromContext(context.Background()) != nil {
		t.Fatal("background context has a span")
	}
}

func TestInjectTraceState(t *testing.T) {
	t.Parallel()

	tracer := New(Destination(&lockedBuffer{}), BatchSize(1))
	t.Cleanup(func() { _ = tracer.Close() })
	span := tracer.StartRemote("request", "request", SpanContext{
		TraceID:    uuid.MustParse("4bf92f35-77b3-4da6-a3ce-929d0e0e4736"),
		SpanID:     uuid.MustParse("00000000-0000-0000-00f0-67aa0ba902b7"),
		Sampled:    true,
		TraceState: testTraceState,
	})
	if span == nil {
		t.Fatal("StartRemote() returned nil")
	}
	header := http.Header{}
	Inject(ContextWithSpan(context.Background(), span), header)
	if got := header.Get("tracestate"); got != testTraceState {
		t.Fatalf("tracestate = %q", got)
	}
}
