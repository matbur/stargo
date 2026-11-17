package sse_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/sse"
)

func TestFirstWriteSendsHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements("<div>x</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if !recorder.Flushed {
		t.Fatal("the event was not flushed")
	}
	if !stream.Started() {
		t.Fatal("stream does not report started")
	}
}

func TestElementPatchBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content any
		opts    []sse.Option
		want    string
	}{
		{
			name:    "simple",
			content: "<div>x</div>",
			want:    "event: datastar-patch-elements\ndata: elements <div>x</div>\n\n",
		},
		{
			name:    "full options",
			content: "<div>x</div>",
			opts: []sse.Option{
				sse.WithEventID("evt-1"),
				sse.WithRetry(2500),
				sse.WithMode(sse.ModeOuter),
				sse.WithSelector("#row-3"),
				sse.WithNamespace(sse.NamespaceSVG),
				sse.WithViewTransition(),
				sse.WithViewTransitionSelector("#panel"),
			},
			want: "event: datastar-patch-elements\n" +
				"id: evt-1\n" +
				"retry: 2500\n" +
				"data: mode outer\n" +
				"data: selector #row-3\n" +
				"data: namespace svg\n" +
				"data: useViewTransition true\n" +
				"data: viewTransitionSelector #panel\n" +
				"data: elements <div>x</div>\n\n",
		},
		{
			name:    "default retry omitted",
			content: "<div>x</div>",
			opts:    []sse.Option{sse.WithRetry(1000)},
			want:    "event: datastar-patch-elements\ndata: elements <div>x</div>\n\n",
		},
		{
			name:    "multiline",
			content: "<div>\n<span>a</span>\r\n<b>b</b>\rc</div>",
			want: "event: datastar-patch-elements\n" +
				"data: elements <div>\n" +
				"data: elements <span>a</span>\n" +
				"data: elements <b>b</b>\n" +
				"data: elements c</div>\n\n",
		},
		{
			name:    "trailing newline",
			content: "a\n",
			want:    "event: datastar-patch-elements\ndata: elements a\ndata: elements \n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			stream := sse.New(recorder)
			if err := stream.PatchElements(test.content, test.opts...); err != nil {
				t.Fatalf("PatchElements() error = %v", err)
			}
			if got := recorder.Body.String(); got != test.want {
				t.Fatalf("body = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSignalPatchBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		signals map[string]any
		opts    []sse.Option
		want    string
	}{
		{
			name:    "dotted key",
			signals: map[string]any{"ui.open": true},
			want:    "event: datastar-patch-signals\ndata: signals {\"ui\":{\"open\":true}}\n\n",
		},
		{
			name:    "nested key",
			signals: map[string]any{"ui": map[string]any{"open": true}},
			want:    "event: datastar-patch-signals\ndata: signals {\"ui\":{\"open\":true}}\n\n",
		},
		{
			name:    "only if missing",
			signals: map[string]any{"n": 1},
			opts:    []sse.Option{sse.WithOnlyIfMissing()},
			want:    "event: datastar-patch-signals\ndata: onlyIfMissing true\ndata: signals {\"n\":1}\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			stream := sse.New(recorder)
			if err := stream.PatchSignals(test.signals, test.opts...); err != nil {
				t.Fatalf("PatchSignals() error = %v", err)
			}
			if got := recorder.Body.String(); got != test.want {
				t.Fatalf("body = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScriptRemoveNavigatePing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		write func(*sse.Stream) error
		want  string
	}{
		{
			name:  "execute script",
			write: func(s *sse.Stream) error { return s.ExecuteScript("alert(1)") },
			want: "event: datastar-patch-elements\n" +
				"data: mode append\n" +
				"data: selector body\n" +
				"data: elements <script data-effect=\"el.remove();\">alert(1)</script>\n\n",
		},
		{
			name: "execute script without removal",
			write: func(s *sse.Stream) error {
				return s.ExecuteScript("alert(1)", sse.WithAutoRemove(false))
			},
			want: "event: datastar-patch-elements\n" +
				"data: mode append\n" +
				"data: selector body\n" +
				"data: elements <script>alert(1)</script>\n\n",
		},
		{
			name:  "remove",
			write: func(s *sse.Stream) error { return s.Remove("#row-3") },
			want: "event: datastar-patch-elements\n" +
				"data: mode remove\n" +
				"data: selector #row-3\n\n",
		},
		{
			name:  "navigate",
			write: func(s *sse.Stream) error { return s.Navigate("/next") },
			want: "event: datastar-patch-elements\n" +
				"data: mode append\n" +
				"data: selector body\n" +
				"data: elements <script data-effect=\"el.remove();\">setTimeout(() => window.location = \"/next\");</script>\n\n",
		},
		{
			name:  "navigate escapes JSON",
			write: func(s *sse.Stream) error { return s.Navigate("/a\"b") },
			want: "event: datastar-patch-elements\n" +
				"data: mode append\n" +
				"data: selector body\n" +
				"data: elements <script data-effect=\"el.remove();\">setTimeout(() => window.location = \"/a\\\"b\");</script>\n\n",
		},
		{
			name:  "ping",
			write: func(s *sse.Stream) error { return s.Ping("keep") },
			want:  ": keep\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			stream := sse.New(recorder)
			if err := test.write(stream); err != nil {
				t.Fatalf("write error = %v", err)
			}
			if got := recorder.Body.String(); got != test.want {
				t.Fatalf("body = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidationWritesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		write func(*sse.Stream) error
	}{
		{"newline in selector", func(s *sse.Stream) error {
			return s.PatchElements("<div>x</div>", sse.WithSelector("a\nb"))
		}},
		{"NUL in event id", func(s *sse.Stream) error {
			return s.PatchElements("<div>x</div>", sse.WithEventID("a\x00b"))
		}},
		{"unknown mode", func(s *sse.Stream) error {
			return s.PatchElements("<div>x</div>", sse.WithMode("sideways"))
		}},
		{"unknown namespace", func(s *sse.Stream) error {
			return s.PatchElements("<div>x</div>", sse.WithNamespace("html"))
		}},
		{"negative retry", func(s *sse.Stream) error {
			return s.PatchElements("<div>x</div>", sse.WithRetry(-1))
		}},
		{"unsupported HTML", func(s *sse.Stream) error {
			return s.PatchElements(42)
		}},
		{"element options on signals", func(s *sse.Stream) error {
			return s.PatchSignals(map[string]any{"a": 1}, sse.WithSelector("#x"))
		}},
		{"newline in removal selector", func(s *sse.Stream) error {
			return s.Remove("a\nb")
		}},
		{"newline in ping", func(s *sse.Stream) error {
			return s.Ping("a\nb")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			stream := sse.New(recorder)
			if err := test.write(stream); err == nil {
				t.Fatal("expected an error")
			}
			if recorder.Body.Len() != 0 {
				t.Fatalf("wrote %q", recorder.Body.String())
			}
			if stream.Started() {
				t.Fatal("stream started despite a validation error")
			}
		})
	}
}

func TestWriteAfterClose(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := stream.PatchElements("<div>x</div>"); err == nil {
		t.Fatal("write after close did not fail")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("wrote %q", recorder.Body.String())
	}
}

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header       { return w.header }
func (w *failingWriter) WriteHeader(status int)    {}
func (w *failingWriter) Write([]byte) (int, error) { return 0, errors.New("client gone") }

func TestFailedWriteDisconnects(t *testing.T) {
	t.Parallel()

	writer := &failingWriter{header: make(http.Header)}
	stream := sse.New(writer)
	err := stream.PatchElements("<div>x</div>")
	if err == nil {
		t.Fatal("expected a write error")
	}
	if !stream.Closed() {
		t.Fatal("stream is not closed after a failed write")
	}
	if stream.Err() == nil {
		t.Fatal("stream does not report the disconnect")
	}
	if err := stream.PatchElements("<div>y</div>"); err == nil {
		t.Fatal("second write did not fail")
	}
}

func TestResponseStartedWithoutEventStream(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements("<div>x</div>"); err != nil {
		t.Fatal(err)
	}
	before := recorder.Body.String()
	recorder.Header().Set("Content-Type", "text/html")

	if err := stream.PatchElements("<div>y</div>"); err == nil {
		t.Fatal("expected an error")
	}
	if got := recorder.Body.String(); got != before {
		t.Fatalf("body changed after the error: %q", got)
	}
}

func TestSequentialPatchesArriveInOrder(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements("<div>first</div>"); err != nil {
		t.Fatal(err)
	}
	if err := stream.PatchElements("<div>second</div>"); err != nil {
		t.Fatal(err)
	}

	body := recorder.Body.String()
	first := strings.Index(body, "first")
	second := strings.Index(body, "second")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("events out of order: %q", body)
	}
	if strings.Count(body, "event: datastar-patch-elements") != 2 {
		t.Fatalf("event count = %d", strings.Count(body, "event: datastar-patch-elements"))
	}
	if !recorder.Flushed {
		t.Fatal("events were not flushed")
	}
}

type contextRenderer struct {
	ctx  context.Context
	body string
}

func (r *contextRenderer) Render(ctx context.Context, w io.Writer) error {
	r.ctx = ctx
	_, err := io.WriteString(w, r.body)
	return err
}

func TestPatchAcceptsContextRenderer(t *testing.T) {
	t.Parallel()

	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "request")
	renderer := &contextRenderer{body: "<div>ctx</div>"}

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder, sse.WithContext(ctx))
	if err := stream.PatchElements(renderer); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if renderer.ctx != ctx {
		t.Fatal("renderer did not receive the stream context")
	}
	if !strings.Contains(recorder.Body.String(), "data: elements <div>ctx</div>") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}
