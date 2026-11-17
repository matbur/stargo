package stargo_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/internal/views"
	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/trace"
)

func newApp(t *testing.T) *stargo.App {
	t.Helper()
	app := stargo.New(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func serve(app *stargo.App, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestRouteRegistrationAndPathValues(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/rooms/{id}"), func(c *stargo.Ctx) error {
		return c.Text(c.R.PathValue("id"))
	})

	recorder := serve(app, http.MethodGet, "/rooms/42")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got != "42" {
		t.Fatalf("body = %q, want 42", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestHandleFunc(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.HandleFunc("GET /hello/{name}", func(c *stargo.Ctx) error {
		return c.Text("hi " + c.R.PathValue("name"))
	})

	recorder := serve(app, http.MethodGet, "/hello/ana")
	if got := recorder.Body.String(); got != "hi ana" {
		t.Fatalf("body = %q", got)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	var order []string
	var mu sync.Mutex
	record := func(value string) {
		mu.Lock()
		order = append(order, value)
		mu.Unlock()
	}

	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record("first")
			next.ServeHTTP(w, r)
			record("first-end")
		})
	})
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record("second")
			next.ServeHTTP(w, r)
			record("second-end")
		})
	})
	app.Handle(route.Get("/x"), func(c *stargo.Ctx) error {
		record("handler")
		return c.NoContent()
	})

	serve(app, http.MethodGet, "/x")

	want := []string{"first", "second", "handler", "second-end", "first-end"}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestSecondCompleteResponse(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	var secondErr error
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error {
		if err := c.Text("first"); err != nil {
			return err
		}
		secondErr = c.Text("second")
		return nil
	})

	recorder := serve(app, http.MethodGet, "/")
	if recorder.Body.String() != "first" {
		t.Fatalf("body = %q, want first", recorder.Body.String())
	}
	if secondErr == nil {
		t.Fatal("second response did not return an error")
	}
}

func TestResponseHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    stargo.Handler
		wantStatus int
		wantType   string
		wantBody   string
		wantHeader string
	}{
		{
			name:       "html",
			handler:    func(c *stargo.Ctx) error { return c.HTML("<b>x</b>") },
			wantStatus: http.StatusOK,
			wantType:   "text/html; charset=utf-8",
			wantBody:   "<b>x</b>",
		},
		{
			name:       "json",
			handler:    func(c *stargo.Ctx) error { return c.JSON(map[string]int{"a": 1}) },
			wantStatus: http.StatusOK,
			wantType:   "application/json",
			wantBody:   `{"a":1}`,
		},
		{
			name:       "text",
			handler:    func(c *stargo.Ctx) error { return c.Text("hello") },
			wantStatus: http.StatusOK,
			wantType:   "text/plain; charset=utf-8",
			wantBody:   "hello",
		},
		{
			name:       "no content",
			handler:    func(c *stargo.Ctx) error { return c.NoContent() },
			wantStatus: http.StatusNoContent,
			wantBody:   "",
		},
		{
			name:       "redirect",
			handler:    func(c *stargo.Ctx) error { return c.Redirect(http.StatusTemporaryRedirect, "/next") },
			wantStatus: http.StatusTemporaryRedirect,
			wantBody:   "",
			wantHeader: "/next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := newApp(t)
			app.Handle(route.Get("/"), test.handler)
			recorder := serve(app, http.MethodGet, "/")

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if test.wantType != "" && recorder.Header().Get("Content-Type") != test.wantType {
				t.Fatalf("Content-Type = %q, want %q", recorder.Header().Get("Content-Type"), test.wantType)
			}
			if test.wantBody != "" && recorder.Body.String() != test.wantBody {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), test.wantBody)
			}
			if test.wantHeader != "" && recorder.Header().Get("Location") != test.wantHeader {
				t.Fatalf("Location = %q, want %q", recorder.Header().Get("Location"), test.wantHeader)
			}
		})
	}
}

func TestNoResponseIsNoContent(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error { return nil })

	if code := serve(app, http.MethodGet, "/").Code; code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", code)
	}
}

func TestErrorStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"typed 404", stargo.NewHTTPError(http.StatusNotFound, "no room"), 404, "no room"},
		{"typed without message", stargo.NewHTTPError(http.StatusBadRequest, ""), 400, http.StatusText(400)},
		{"generic", errors.New("secret detail"), 500, "Internal Server Error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := newApp(t)
			app.Handle(route.Get("/"), func(c *stargo.Ctx) error { return test.err })
			recorder := serve(app, http.MethodGet, "/")

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != test.wantBody {
				t.Fatalf("body = %q, want %q", got, test.wantBody)
			}
			if strings.Contains(recorder.Body.String(), "secret detail") {
				t.Fatal("internal error leaked into the response")
			}
		})
	}
}

func TestErrorAfterResponseStartedAborts(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error {
		if err := c.Text("partial"); err != nil {
			return err
		}
		return errors.New("late failure")
	})

	recorder := httptest.NewRecorder()
	panicked := catchAbort(func() {
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	})
	if !panicked {
		t.Fatal("expected http.ErrAbortHandler")
	}
	if got := recorder.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}

func catchAbort(fn func()) (caught bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			caught = recovered == http.ErrAbortHandler
			if !caught {
				panic(recovered)
			}
		}
	}()
	fn()
	return false
}

func TestPanicBeforeResponse(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error {
		panic("boom")
	})

	recorder := serve(app, http.MethodGet, "/")
	if recorder.Code != 500 {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Internal Server Error") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestPanicAfterResponseStartedAborts(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error {
		if err := c.Text("partial"); err != nil {
			return err
		}
		panic("late panic")
	})

	recorder := httptest.NewRecorder()
	if !catchAbort(func() {
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}) {
		t.Fatal("expected http.ErrAbortHandler")
	}
	if got := recorder.Body.String(); got != "partial" {
		t.Fatalf("body = %q, want partial", got)
	}
}

func TestUnknownPathAndWrongMethod(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/rooms/{id}"), func(c *stargo.Ctx) error { return c.NoContent() })

	if code := serve(app, http.MethodGet, "/nope").Code; code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", code)
	}
	if code := serve(app, http.MethodPost, "/rooms/42").Code; code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status = %d, want 405", code)
	}
}

func TestMethodlessRouteAcceptsEveryMethod(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Any("/any"), func(c *stargo.Ctx) error {
		return c.Text(c.R.Method)
	})

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		if got := serve(app, method, "/any").Body.String(); got != method {
			t.Fatalf("%s body = %q", method, got)
		}
	}
}

func TestRenderingIntegration(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error { return c.HTML(views.Page(0, "/live")) })

	recorder := serve(app, http.MethodGet, "/")
	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "<h1>Stargo counter</h1>") {
		t.Fatalf("body = %q", body)
	}
	if !strings.HasPrefix(body, "<!doctype html>") {
		t.Fatalf("missing doctype: %q", body)
	}
}

func TestRenderErrorsBecome500(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
	}{
		{"unsupported", 42},
		{"failing renderer", failingRenderer{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := newApp(t)
			app.Handle(route.Get("/"), func(c *stargo.Ctx) error { return c.HTML(test.value) })
			recorder := serve(app, http.MethodGet, "/")
			if recorder.Code != 500 {
				t.Fatalf("status = %d, want 500", recorder.Code)
			}
		})
	}
}

type failingRenderer struct{}

func (failingRenderer) Render(ctx context.Context, w io.Writer) error {
	return errors.New("render failed")
}

func TestStreamAfterResponseStarted(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	var streamErr error
	app.Handle(route.Get("/"), func(c *stargo.Ctx) error {
		if err := c.Text("already"); err != nil {
			return err
		}
		_, streamErr = c.Stream()
		return nil
	})

	serve(app, http.MethodGet, "/")
	if streamErr == nil {
		t.Fatal("Stream() did not fail after the response started")
	}
}

// spanRecord mirrors the exported NDJSON shape.
type spanRecord struct {
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

func newTracingApp(t *testing.T) (*stargo.App, *bytes.Buffer) {
	t.Helper()
	buffer := &bytes.Buffer{}
	tracer := trace.New(
		trace.Destination(buffer),
		trace.BatchSize(1),
		trace.FlushInterval(time.Millisecond),
	)
	app := stargo.New(stargo.WithTracer(tracer))
	t.Cleanup(func() { _ = app.Close() })
	return app, buffer
}

func readSpans(t *testing.T, buffer *bytes.Buffer) []spanRecord {
	t.Helper()
	var records []spanRecord
	scanner := bufio.NewScanner(bytes.NewReader(buffer.Bytes()))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record spanRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("invalid span line %q: %v", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan spans: %v", err)
	}
	return records
}

func TestRequestSpan(t *testing.T) {
	t.Parallel()

	app, buffer := newTracingApp(t)
	app.Handle(route.Get("/rooms/{id}"), func(c *stargo.Ctx) error {
		c.Span.Attr("handler", "room")
		return c.Text("ok")
	})

	serve(app, http.MethodGet, "/rooms/42")
	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	records := readSpans(t, buffer)
	if len(records) != 1 {
		t.Fatalf("got %d spans, want 1", len(records))
	}
	record := records[0]
	if record.Name != "GET /rooms/{id}" {
		t.Fatalf("name = %q", record.Name)
	}
	if record.Kind != "request" {
		t.Fatalf("kind = %q", record.Kind)
	}
	if record.Status != "ok" {
		t.Fatalf("status = %q", record.Status)
	}
	if record.StartNS == 0 || record.EndNS < record.StartNS || record.DurationNS < 0 {
		t.Fatalf("bad timing: %+v", record)
	}
	wantAttributes := map[string]any{
		"http.route":  "GET /rooms/{id}",
		"http.method": "GET",
		"http.path":   "/rooms/42",
		"http.status": float64(200),
		"handler":     "room",
	}
	for key, want := range wantAttributes {
		if got := record.Attributes[key]; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("attribute %s = %v, want %v", key, got, want)
		}
	}
}

func TestErrorAndPanicSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		handler   stargo.Handler
		wantEvent string
	}{
		{
			name: "handler error",
			handler: func(c *stargo.Ctx) error {
				return errors.New("boom")
			},
			wantEvent: "exception",
		},
		{
			name: "panic",
			handler: func(c *stargo.Ctx) error {
				panic("boom")
			},
			wantEvent: "panic",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app, buffer := newTracingApp(t)
			app.Handle(route.Get("/"), test.handler)
			serve(app, http.MethodGet, "/")
			if err := app.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			records := readSpans(t, buffer)
			if len(records) != 1 {
				t.Fatalf("got %d spans, want 1", len(records))
			}
			record := records[0]
			if record.Status != "error" {
				t.Fatalf("status = %q", record.Status)
			}
			if !strings.Contains(record.Error, "boom") {
				t.Fatalf("error = %q", record.Error)
			}
			found := false
			for _, event := range record.Events {
				if event.Name == test.wantEvent {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %q event in %+v", test.wantEvent, record.Events)
			}
		})
	}
}

func TestStreamSpanKindAndClientLeft(t *testing.T) {
	t.Parallel()

	app, buffer := newTracingApp(t)
	entered := make(chan struct{})

	app.Handle(route.Get("/live"), func(c *stargo.Ctx) error {
		stream, err := c.Stream()
		if err != nil {
			return err
		}
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			return err
		}
		close(entered)
		<-c.R.Context().Done()
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		app.ServeHTTP(recorder, req)
		close(done)
	}()

	<-entered
	cancel()
	<-done

	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	records := readSpans(t, buffer)
	if len(records) != 1 {
		t.Fatalf("got %d spans, want 1", len(records))
	}
	record := records[0]
	if record.Kind != "stream" {
		t.Fatalf("kind = %q, want stream", record.Kind)
	}
	if record.Status != "ok" {
		t.Fatalf("status = %q, want ok", record.Status)
	}
	if got := record.Attributes["request.client_left"]; got != true {
		t.Fatalf("request.client_left = %v", got)
	}
}

func TestUnmatchedRequestSpan(t *testing.T) {
	t.Parallel()

	app, buffer := newTracingApp(t)
	app.Handle(route.Get("/x"), func(c *stargo.Ctx) error { return c.NoContent() })

	serve(app, http.MethodGet, "/missing")
	serve(app, http.MethodPost, "/x")
	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	records := readSpans(t, buffer)
	if len(records) != 2 {
		t.Fatalf("got %d spans, want 2", len(records))
	}
	statuses := map[string]bool{}
	for _, record := range records {
		if record.Name != "unmatched" {
			t.Fatalf("name = %q", record.Name)
		}
		statuses[fmt.Sprint(record.Attributes["http.status"])] = true
	}
	if !statuses["404"] || !statuses["405"] {
		t.Fatalf("statuses = %v, want 404 and 405", statuses)
	}
}
