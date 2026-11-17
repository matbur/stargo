package trace_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matbur/stargo/trace"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type record struct {
	SpanID     string         `json:"span_id"`
	TraceID    string         `json:"trace_id"`
	ParentID   string         `json:"parent_id"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	StartNS    int64          `json:"start_ns"`
	EndNS      int64          `json:"end_ns"`
	DurationNS int64          `json:"duration_ns"`
	Status     string         `json:"status"`
	Error      string         `json:"error"`
	Attributes map[string]any `json:"attributes"`
	Events     []struct {
		TimeNS     int64          `json:"time_ns"`
		Name       string         `json:"name"`
		Attributes map[string]any `json:"attributes"`
	} `json:"events"`
}

func newTracer(t *testing.T, buffer *syncBuffer, opts ...trace.Option) *trace.Tracer {
	t.Helper()
	options := append([]trace.Option{
		trace.Destination(buffer),
		trace.BatchSize(1),
		trace.FlushInterval(time.Millisecond),
	}, opts...)
	tracer := trace.New(options...)
	t.Cleanup(func() { _ = tracer.Close() })
	return tracer
}

func readRecords(t *testing.T, content string) []record {
	t.Helper()
	var records []record
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var rec record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("invalid line %q: %v", scanner.Text(), err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return records
}

func TestSpanTreeAndAttributes(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)

	parent := tracer.Start("request", "request")
	parent.Attr("http.method", "GET")
	parent.Event("started", map[string]any{"detail": "go"})

	child, err := parent.Step("db.load")
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	child.Attr("db", "main")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	err = child.Do(cancelled, "query", func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	child.End()
	parent.End()

	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	records := readRecords(t, buffer.String())
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}

	byName := map[string]record{}
	for _, rec := range records {
		byName[rec.Name] = rec
	}
	root := byName["request"]
	if root.TraceID != root.SpanID {
		t.Fatalf("root trace id %q != span id %q", root.TraceID, root.SpanID)
	}
	if root.ParentID != "" {
		t.Fatalf("root parent id = %q", root.ParentID)
	}
	if root.Kind != "request" || root.Status != "ok" {
		t.Fatalf("root = %+v", root)
	}
	if root.Attributes["http.method"] != "GET" {
		t.Fatalf("root attributes = %v", root.Attributes)
	}
	if len(root.Events) != 1 || root.Events[0].Name != "started" {
		t.Fatalf("root events = %+v", root.Events)
	}
	if root.Events[0].Attributes["detail"] != "go" {
		t.Fatalf("event attributes = %v", root.Events[0].Attributes)
	}

	step := byName["db.load"]
	if step.TraceID != root.TraceID || step.ParentID != root.SpanID {
		t.Fatalf("child does not reference the parent: %+v", step)
	}
	if step.Kind != "request" {
		t.Fatalf("child kind = %q", step.Kind)
	}

	query := byName["query"]
	if query.ParentID != step.SpanID || query.TraceID != root.TraceID {
		t.Fatalf("nested child is wrong: %+v", query)
	}
}

func TestStepOnEndedSpan(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	span := tracer.Start("request", "request")
	span.End()
	if _, err := span.Step("late"); err == nil {
		t.Fatal("Step() on an ended span did not fail")
	}
}

func TestNilSpanAndIdempotentEnd(t *testing.T) {
	t.Parallel()

	var span *trace.Span
	span.Attr("a", 1)
	span.Event("e", nil)
	span.Fail(errors.New("x"))
	span.SetKind("stream")
	span.End()
	child, err := span.Step("child")
	if err != nil || child != nil {
		t.Fatalf("nil Step() = %v, %v", child, err)
	}

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	real := tracer.Start("request", "request")
	real.End()
	real.End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	if got := len(readRecords(t, buffer.String())); got != 1 {
		t.Fatalf("got %d records, want 1", got)
	}
}

func TestUnclosedStepIsNotExported(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	parent := tracer.Start("request", "request")
	if _, err := parent.Step("never-ended"); err != nil {
		t.Fatal(err)
	}
	parent.End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}

	records := readRecords(t, buffer.String())
	if len(records) != 1 || records[0].Name != "request" {
		t.Fatalf("records = %+v", records)
	}
}

func TestFailAndIgnoreAfterEnd(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	span := tracer.Start("request", "request")
	span.Attr("before", 1)
	span.Fail(errors.New("boom"))
	span.End()
	span.Attr("after", 2)
	span.Event("after", nil)
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}

	records := readRecords(t, buffer.String())
	if len(records) != 1 {
		t.Fatalf("got %d records", len(records))
	}
	rec := records[0]
	if rec.Status != "error" || rec.Error != "boom" {
		t.Fatalf("status = %q error = %q", rec.Status, rec.Error)
	}
	if _, ok := rec.Attributes["after"]; ok {
		t.Fatal("attribute was recorded after End")
	}
	if len(rec.Events) != 0 {
		t.Fatal("event was recorded after End")
	}
}

func TestEventCap(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	span := tracer.Start("request", "request")
	for i := 0; i < 1005; i++ {
		span.Event("tick", map[string]any{"i": i})
	}
	span.End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}

	records := readRecords(t, buffer.String())
	if len(records) != 1 {
		t.Fatalf("got %d records", len(records))
	}
	if len(records[0].Events) != 1000 {
		t.Fatalf("got %d events, want 1000", len(records[0].Events))
	}
	if got := records[0].Attributes["span.dropped_events"]; got != float64(5) {
		t.Fatalf("span.dropped_events = %v, want 5", got)
	}
}

func TestBatchSizeFlush(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := trace.New(
		trace.Destination(buffer),
		trace.BatchSize(2),
		trace.FlushInterval(time.Hour),
	)
	t.Cleanup(func() { _ = tracer.Close() })

	tracer.Start("one", "request").End()
	time.Sleep(20 * time.Millisecond)
	if got := buffer.String(); got != "" {
		t.Fatalf("flushed before the batch was full: %q", got)
	}

	tracer.Start("two", "request").End()
	waitFor(t, func() bool { return strings.Count(buffer.String(), "\n") == 2 }, time.Second)
}

func TestIntervalFlush(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := trace.New(
		trace.Destination(buffer),
		trace.BatchSize(100),
		trace.FlushInterval(10*time.Millisecond),
	)
	t.Cleanup(func() { _ = tracer.Close() })

	tracer.Start("one", "request").End()
	waitFor(t, func() bool { return strings.Contains(buffer.String(), `"name":"one"`) }, time.Second)
}

func waitFor(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

type blockingWriter struct {
	block chan struct{}
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	<-w.block
	return len(p), nil
}

func TestFullQueueDropsSpans(t *testing.T) {
	t.Parallel()

	writer := &blockingWriter{block: make(chan struct{})}
	tracer := trace.New(
		trace.Destination(writer),
		trace.QueueSize(1),
		trace.BatchSize(1),
		trace.FlushInterval(time.Millisecond),
	)

	for i := 0; i < 10; i++ {
		tracer.Start("span", "request").End()
	}
	waitFor(t, func() bool { return tracer.DroppedSpans() > 0 }, time.Second)

	close(writer.block)
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if tracer.DroppedSpans() == 0 {
		t.Fatal("drop counter did not increase")
	}
}

func TestCloseDrainsQueue(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := trace.New(
		trace.Destination(buffer),
		trace.QueueSize(100),
		trace.BatchSize(1000),
		trace.FlushInterval(time.Hour),
	)
	for i := 0; i < 5; i++ {
		tracer.Start("span", "request").End()
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := strings.Count(buffer.String(), "\n"); got != 5 {
		t.Fatalf("drained %d lines, want 5", got)
	}
}

func TestRotation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")
	tracer := trace.New(
		trace.File(path),
		trace.Rotate(400, 3),
		trace.BatchSize(1),
		trace.FlushInterval(time.Millisecond),
	)
	for i := 0; i < 20; i++ {
		span := tracer.Start("request", "request")
		span.Attr("payload", strings.Repeat("x", 100))
		span.End()
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("rotation kept %d files, want 3", len(entries))
	}
	if _, err := os.Stat(path + ".3"); err == nil {
		t.Fatal("rotation kept more files than configured")
	}

	total := 0
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		total += len(readRecords(t, string(content)))
	}
	if total == 0 {
		t.Fatal("rotation lost every span")
	}
}

func TestFileDestinationFromEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.ndjson")
	t.Setenv("TRACE", path)
	t.Setenv("TRACE_FILE", "")
	t.Setenv("TRACE_MAX_MB", "")
	t.Setenv("TRACE_KEEP", "")

	tracer := trace.FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("request", "request").End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(readRecords(t, string(content))) != 1 {
		t.Fatalf("file content = %q", content)
	}
}

func TestTraceFileEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.ndjson")
	t.Setenv("TRACE", "1")
	t.Setenv("TRACE_FILE", path)
	t.Setenv("TRACE_MAX_MB", "")
	t.Setenv("TRACE_KEEP", "")

	tracer := trace.FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("request", "request").End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("TRACE_FILE was not used: %v", err)
	}
}

func TestStdoutDestination(t *testing.T) {
	old := os.Stdout
	readSide, writeSide, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writeSide
	t.Cleanup(func() {
		os.Stdout = old
		_ = writeSide.Close()
		_ = readSide.Close()
	})

	tracer := trace.New(trace.Stdout(), trace.BatchSize(1), trace.FlushInterval(time.Millisecond))
	tracer.Start("request", "request").End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeSide.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(readSide)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"name":"request"`) {
		t.Fatalf("stdout = %q", content)
	}
}

func TestDisabledTracing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TRACE", "")
	t.Setenv("TRACE_FILE", "")

	if tracer := trace.FromEnv(); tracer != nil {
		t.Fatal("FromEnv() returned a tracer while tracing is off")
	}

	var tracer *trace.Tracer
	span := tracer.Start("request", "request")
	if span != nil {
		t.Fatal("nil tracer returned a span")
	}
	span.Attr("a", 1)
	span.Event("e", nil)
	span.End()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("disabled tracing wrote files: %v", entries)
	}
}

func TestStdoutEnv(t *testing.T) {
	old := os.Stdout
	readSide, writeSide, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writeSide
	t.Cleanup(func() {
		os.Stdout = old
		_ = writeSide.Close()
		_ = readSide.Close()
	})
	t.Setenv("TRACE", "-")
	t.Setenv("TRACE_FILE", "")
	t.Setenv("TRACE_MAX_MB", "")

	tracer := trace.FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("request", "request").End()
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeSide.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(readSide)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"name":"request"`) {
		t.Fatalf("stdout = %q", content)
	}
}

func TestStartAfterClose(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	if span := tracer.Start("late", "request"); span != nil {
		t.Fatal("Start() after Close returned a span")
	}
}

func TestDoPropagatesPanic(t *testing.T) {
	t.Parallel()

	buffer := &syncBuffer{}
	tracer := newTracer(t, buffer)
	parent := tracer.Start("request", "request")
	defer parent.End()

	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic was swallowed")
			}
		}()
		_ = parent.Do(context.Background(), "boom", func(ctx context.Context) error {
			panic("explode")
		})
	}()

	if err := tracer.Close(); err != nil {
		t.Fatal(err)
	}
	records := readRecords(t, buffer.String())
	var child *record
	for i := range records {
		if records[i].Name == "boom" {
			child = &records[i]
		}
	}
	if child == nil {
		t.Fatalf("child span missing from %+v", records)
	}
	if child.Status != "error" {
		t.Fatalf("child status = %q", child.Status)
	}
	found := false
	for _, event := range child.Events {
		if event.Name == "panic" {
			found = true
		}
	}
	if !found {
		t.Fatalf("panic event missing: %+v", child.Events)
	}
}
