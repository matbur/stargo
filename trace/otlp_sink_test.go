package trace

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type otlpRequest struct {
	path    string
	header  http.Header
	payload otlpPayload
}

func collector(t *testing.T, requests chan<- otlpRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload otlpPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- otlpRequest{path: r.URL.Path, header: r.Header.Clone(), payload: payload}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

func waitOTLP(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func sampleRecord(name string) record {
	return record{
		TraceID: uuid.NewV7().String(),
		SpanID:  uuid.NewV7().String(),
		Name:    name,
		Kind:    "request",
		StartNS: 1,
		EndNS:   2,
		Status:  "ok",
	}
}

func TestOTLPSinkPostsPayload(t *testing.T) {
	requests := make(chan otlpRequest, 4)
	server := collector(t, requests)
	t.Chdir(t.TempDir())

	tracer := New(
		OTLP(server.URL),
		BatchSize(1),
		FlushInterval(time.Millisecond),
	)
	t.Cleanup(func() { _ = tracer.Close() })

	tracer.Start("GET /rooms", "request").End()

	select {
	case request := <-requests:
		if request.path != "/v1/traces" {
			t.Fatalf("path = %q, want /v1/traces", request.path)
		}
		if got := request.header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		spans := request.payload.ResourceSpans[0].ScopeSpans[0].Spans
		if len(spans) != 1 || spans[0].Name != "GET /rooms" {
			t.Fatalf("spans = %+v", spans)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector received no request")
	}

	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat("trace.ndjson"); err == nil {
		t.Fatal("OTLP-only tracing wrote a local file")
	}
}

func TestOTLPSinkRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := defaults()
	cfg.batchSize = 1
	cfg.flushInterval = time.Millisecond
	sink := newOTLPSink(otlpConfig{
		endpoint:      server.URL,
		timeout:       time.Second,
		retryAttempts: 3,
		retryDelay:    time.Millisecond,
	}, cfg, "svc")
	t.Cleanup(func() { _ = sink.close() })

	sink.enqueue(sampleRecord("retry"))
	waitOTLP(t, func() bool { return calls.Load() == 2 })
	if err := sink.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}
	if got := sink.dropped(); got != 0 {
		t.Fatalf("dropped = %d, want 0", got)
	}
}

func TestOTLPSinkDoesNotRetryPermanentFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	cfg := defaults()
	cfg.batchSize = 1
	cfg.flushInterval = time.Millisecond
	sink := newOTLPSink(otlpConfig{
		endpoint:      server.URL,
		timeout:       time.Second,
		retryAttempts: 3,
		retryDelay:    time.Millisecond,
	}, cfg, "svc")

	sink.enqueue(sampleRecord("reject"))
	waitOTLP(t, func() bool { return sink.dropped() == 1 })
	if err := sink.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestOTLPSinkDropsWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := defaults()
	cfg.queueSize = 1
	cfg.batchSize = 1
	cfg.flushInterval = time.Millisecond
	sink := newOTLPSink(otlpConfig{
		endpoint:      server.URL,
		timeout:       time.Second,
		retryAttempts: 1,
		retryDelay:    time.Millisecond,
	}, cfg, "svc")

	for range 10 {
		sink.enqueue(sampleRecord("full"))
	}
	waitOTLP(t, func() bool { return sink.dropped() > 0 })

	close(release)
	if err := sink.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}
}

func TestOTLPSinkFlushesOnClose(t *testing.T) {
	t.Parallel()

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)

	cfg := defaults()
	cfg.queueSize = 10
	cfg.batchSize = 100
	cfg.flushInterval = time.Hour
	sink := newOTLPSink(otlpConfig{
		endpoint:      server.URL,
		timeout:       time.Second,
		retryAttempts: 1,
	}, cfg, "svc")

	for range 3 {
		sink.enqueue(sampleRecord("close"))
	}
	if err := sink.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}

	select {
	case request := <-requests:
		spans := request.payload.ResourceSpans[0].ScopeSpans[0].Spans
		if len(spans) != 3 {
			t.Fatalf("spans on close = %d, want 3", len(spans))
		}
	default:
		t.Fatal("close did not flush the batch")
	}
}

func TestOTLPWithLocalDestination(t *testing.T) {
	t.Parallel()

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	buffer := &lockedBuffer{}

	tracer := New(
		Destination(buffer),
		OTLP(server.URL),
		OTLPHeaders(map[string]string{"X-Scope": "test"}),
		BatchSize(1),
		FlushInterval(time.Millisecond),
	)

	tracer.Start("both", "request").End()

	select {
	case request := <-requests:
		if got := request.header.Get("X-Scope"); got != "test" {
			t.Fatalf("header = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector received no request")
	}
	waitOTLP(t, func() bool { return buffer.String() != "" })
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
