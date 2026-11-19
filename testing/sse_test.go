package stargotest_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	stargotest "github.com/matbur/stargo/testing"
)

func eventStreamHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	})
}

func openStream(t *testing.T, client *stargotest.Client) *stargotest.Stream {
	t.Helper()
	stream := client.Open(client.Request(http.MethodGet, "/events"))
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func TestStreamHeadersBeforeFirstEvent(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "event: ping\ndata: ready\n\n")
	})

	client := stargotest.NewClient(t, handler)
	stream := openStream(t, client)
	closed := false
	closeRelease := func() {
		if !closed {
			closed = true
			close(release)
		}
	}
	defer closeRelease()

	if stream.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", stream.StatusCode)
	}
	if got := stream.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := stream.Header.Get("X-Test"); got != "yes" {
		t.Fatalf("X-Test = %q", got)
	}

	closeRelease()
	event, ok := stream.Next()
	if !ok {
		t.Fatal("Next() returned no event")
	}
	if event.Type != "ping" || event.Data != "ready" {
		t.Fatalf("event = %+v", event)
	}
}

func TestStreamCloseStopsReads(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: first\ndata: one\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})

	client := stargotest.NewClient(t, handler)
	stream := client.Open(client.Request(http.MethodGet, "/events"))

	if _, ok := stream.Next(); !ok {
		t.Fatal("Next() returned no event before close")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, ok := stream.Next(); ok {
		t.Fatal("Next() returned an event after close")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestStreamParsesEvents(t *testing.T) {
	t.Parallel()

	body := ": hello\r\n" +
		"event: update\r\n" +
		"id: 7\r\n" +
		"retry: 3000\r\n" +
		"data: first\r\n" +
		"data: second\r\n" +
		"\r\n" +
		"event: ping\n" +
		"data: one\n" +
		"\n" +
		"data: partial"

	client := stargotest.NewClient(t, eventStreamHandler(body))
	stream := openStream(t, client)

	first, ok := stream.Next()
	if !ok {
		t.Fatal("first Next() returned no event")
	}
	if first.Type != "update" {
		t.Fatalf("Type = %q, want update", first.Type)
	}
	if first.ID != "7" {
		t.Fatalf("ID = %q, want 7", first.ID)
	}
	if first.Retry != 3*time.Second {
		t.Fatalf("Retry = %v, want 3s", first.Retry)
	}
	if first.Data != "first\nsecond" {
		t.Fatalf("Data = %q, want joined data lines", first.Data)
	}
	if len(first.Comments) != 1 || first.Comments[0] != "hello" {
		t.Fatalf("Comments = %v, want [hello]", first.Comments)
	}

	second, ok := stream.Next()
	if !ok {
		t.Fatal("second Next() returned no event")
	}
	if second.Type != "ping" || second.Data != "one" {
		t.Fatalf("second event = %+v", second)
	}

	third, ok := stream.Next()
	if !ok {
		t.Fatal("third Next() returned no event")
	}
	if third.Data != "partial" {
		t.Fatalf("third Data = %q, want partial", third.Data)
	}

	if _, ok := stream.Next(); ok {
		t.Fatal("Next() returned an event after the stream ended")
	}
}

func TestStreamParsesCommentOnlyBlock(t *testing.T) {
	t.Parallel()

	client := stargotest.NewClient(t, eventStreamHandler(": only a comment\n\n"))
	stream := openStream(t, client)

	event, ok := stream.Next()
	if !ok {
		t.Fatal("Next() returned no event")
	}
	if event.Type != "" || event.Data != "" {
		t.Fatalf("event = %+v, want no type or data", event)
	}
	if len(event.Comments) != 1 || event.Comments[0] != "only a comment" {
		t.Fatalf("Comments = %v", event.Comments)
	}
}

func TestStreamHandlesLargePayload(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("x", 70_000)
	client := stargotest.NewClient(t, eventStreamHandler("event: big\ndata: "+payload+"\n\n"))
	stream := openStream(t, client)

	event, ok := stream.Next()
	if !ok {
		t.Fatal("Next() returned no event")
	}
	if event.Data != payload {
		t.Fatalf("Data length = %d, want %d", len(event.Data), len(payload))
	}
}

func TestElementPatchParsing(t *testing.T) {
	t.Parallel()

	event := stargotest.Event{
		Type: "datastar-patch-elements",
		Data: "mode append\n" +
			"selector #feed\n" +
			"namespace svg\n" +
			"useViewTransition true\n" +
			"viewTransitionSelector #feed\n" +
			"elements <div>one</div>\n" +
			"elements <div>two</div>",
	}

	patch, err := event.Patch()
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patch.Kind != stargotest.PatchElements {
		t.Fatalf("Kind = %v, want elements", patch.Kind)
	}
	if patch.Mode != "append" || patch.Selector != "#feed" || patch.Namespace != "svg" {
		t.Fatalf("patch metadata = %+v", patch)
	}
	if !patch.UseViewTransition || patch.ViewTransitionSelector != "#feed" {
		t.Fatalf("view transition = %+v", patch)
	}
	if patch.Elements != "<div>one</div>\n<div>two</div>" {
		t.Fatalf("Elements = %q", patch.Elements)
	}
}

func TestSignalPatchParsing(t *testing.T) {
	t.Parallel()

	event := stargotest.Event{
		Type: "datastar-patch-signals",
		Data: "onlyIfMissing true\nsignals {\"count\":2,\"name\":\"Ada\"}",
	}

	patch, err := event.Patch()
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patch.Kind != stargotest.PatchSignals {
		t.Fatalf("Kind = %v, want signals", patch.Kind)
	}
	if !patch.OnlyIfMissing {
		t.Fatal("OnlyIfMissing = false, want true")
	}
	if patch.Signals["name"] != "Ada" || patch.Signals["count"] != float64(2) {
		t.Fatalf("Signals = %+v", patch.Signals)
	}
}

func TestPatchRejectsOtherEvents(t *testing.T) {
	t.Parallel()

	_, err := (stargotest.Event{Type: "ping"}).Patch()
	if err != stargotest.ErrNotPatch {
		t.Fatalf("Patch() error = %v, want ErrNotPatch", err)
	}
}

func TestNextPatchSkipsOtherEvents(t *testing.T) {
	t.Parallel()

	body := "event: ping\ndata: keepalive\n\n" +
		"event: datastar-patch-elements\n" +
		"data: mode append\n" +
		"data: selector #feed\n" +
		"data: elements <p>hello</p>\n\n"

	client := stargotest.NewClient(t, eventStreamHandler(body))
	stream := openStream(t, client)

	patch, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no patch")
	}
	if patch.Kind != stargotest.PatchElements || patch.Mode != "append" || patch.Elements != "<p>hello</p>" {
		t.Fatalf("patch = %+v", patch)
	}
	if _, ok := stream.NextPatch(); ok {
		t.Fatal("NextPatch() returned a patch after the stream ended")
	}
}

func TestNextPatchReportsInvalidSignals(t *testing.T) {
	t.Parallel()

	body := "event: datastar-patch-signals\ndata: signals {bad json\n\n"

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, eventStreamHandler(body))
		stream := client.Open(client.Request(http.MethodGet, "/events"))
		stream.NextPatch()
	})

	for _, want := range []string{"parse SSE patch", "signals {bad json"} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure = %q, want %q", message, want)
		}
	}
}

func TestPatchKindString(t *testing.T) {
	t.Parallel()

	if got := fmt.Sprint(stargotest.PatchElements); got != "datastar-patch-elements" {
		t.Fatalf("PatchElements = %q", got)
	}
	if got := fmt.Sprint(stargotest.PatchSignals); got != "datastar-patch-signals" {
		t.Fatalf("PatchSignals = %q", got)
	}
}
