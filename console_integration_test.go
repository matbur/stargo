package stargo

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matbur/stargo/internal/console"
	"github.com/matbur/stargo/route"
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

func (b *lockedBuffer) Contains(text string) bool {
	return strings.Contains(b.String(), text)
}

func TestDisplayName(t *testing.T) {
	t.Parallel()

	app := New(WithTracer(nil))
	if got := app.displayName(); got == "" {
		t.Fatal("default display name is empty")
	}
	named := New(WithTracer(nil), WithName("console-test"))
	if got := named.displayName(); got != "console-test" {
		t.Fatalf("displayName() = %q, want console-test", got)
	}
}

func TestServeWritesStartupBlock(t *testing.T) {
	t.Parallel()

	app := New(WithTracer(nil), WithName("console-test"))
	listener := listen(t)
	defer listener.Close()
	buffer := &lockedBuffer{}

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errCh <- app.Serve(ctx, listener, Console(buffer), ShutdownTimeout(3*time.Second))
	}()

	waitFor(t, func() bool { return buffer.Contains("server.startup") }, time.Second, "startup block missing")
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	text := buffer.String()
	header := regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d ● server\.startup`).FindString(text)
	if header == "" {
		t.Fatalf("startup header missing: %q", text)
	}
	for _, want := range []string{
		"app.name=console-test",
		"listen_mode=tcp",
		"host=127.0.0.1",
		"port=",
		"tls=false",
		"listening=true",
		"graceful_shutdown_timeout=3s",
		"timeout  request_header=5s keep_alive=2m0s",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("startup block missing %q: %q", want, text)
		}
	}
}

func TestServeHTTPWritesRequestLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		target     string
		register   func(*App)
		wantMarker string
		wantStatus int
	}{
		{
			name:       "success",
			method:     http.MethodGet,
			target:     "/",
			register:   func(app *App) { app.Handle(route.Get("/"), func(c *Ctx) error { return c.Text("ok") }) },
			wantMarker: "●",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			target:     "/missing",
			register:   func(app *App) { app.Handle(route.Get("/only"), func(c *Ctx) error { return c.Text("ok") }) },
			wantMarker: "●",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			target:     "/only",
			register:   func(app *App) { app.Handle(route.Get("/only"), func(c *Ctx) error { return c.Text("ok") }) },
			wantMarker: "●",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "failure",
			method: http.MethodGet,
			target: "/boom",
			register: func(app *App) {
				app.Handle(route.Get("/boom"), func(c *Ctx) error { return errors.New("boom") })
			},
			wantMarker: "×",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			app := New(WithTracer(nil))
			test.register(app)
			buffer := &lockedBuffer{}
			app.console.Store(console.NewWriter(buffer, 100, false))

			app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(test.method, test.target, nil))

			lines := requestLines(buffer.String())
			if len(lines) != 1 {
				t.Fatalf("got %d lines, want 1: %q", len(lines), buffer.String())
			}
			line := lines[0]
			prefix := test.wantMarker + " " + test.method + " " + test.target + "  " + strconv.Itoa(test.wantStatus)
			if !strings.Contains(line, prefix) {
				t.Fatalf("line = %q, want %q", line, prefix)
			}
		})
	}
}

func TestStreamRequestLinesAtCompletion(t *testing.T) {
	t.Parallel()

	app := New(WithTracer(nil))
	app.Handle(route.Get("/live"), func(c *Ctx) error {
		stream, err := c.Stream()
		if err != nil {
			return err
		}
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
		return nil
	})
	buffer := &lockedBuffer{}
	app.console.Store(console.NewWriter(buffer, 100, false))

	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/live", nil))

	lines := requestLines(buffer.String())
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1: %q", len(lines), buffer.String())
	}
	if !strings.Contains(lines[0], "● GET /live  200") {
		t.Fatalf("line = %q", lines[0])
	}
	if elapsed := durationSuffix(lines[0]); elapsed < 20*time.Millisecond {
		t.Fatalf("duration = %v, want at least 20ms", elapsed)
	}
}

func TestConcurrentRequestLinesAreWholeBlocks(t *testing.T) {
	t.Parallel()

	app := New(WithTracer(nil))
	app.Handle(route.Get("/"), func(c *Ctx) error { return c.NoContent() })
	buffer := &lockedBuffer{}
	app.console.Store(console.NewWriter(buffer, 100, false))

	const count = 20
	var group sync.WaitGroup
	group.Add(count)
	for range count {
		go func() {
			defer group.Done()
			app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		}()
	}
	group.Wait()

	lines := requestLines(buffer.String())
	if len(lines) != count {
		t.Fatalf("got %d lines, want %d: %q", len(lines), count, buffer.String())
	}
	for _, line := range lines {
		if !strings.Contains(line, "● GET /  204") {
			t.Fatalf("incomplete line: %q", line)
		}
	}
}

func TestServeWritesShutdownBlock(t *testing.T) {
	t.Parallel()

	app := New(WithTracer(nil), WithName("console-test"))
	app.Handle(route.Get("/"), func(c *Ctx) error { return c.Text("ok") })
	listener := listen(t)
	defer listener.Close()
	buffer := &lockedBuffer{}

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errCh <- app.Serve(ctx, listener, Console(buffer))
	}()

	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = response.Body.Close()

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	text := buffer.String()
	if !regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d ● server\.shutdown`).MatchString(text) {
		t.Fatalf("shutdown header missing: %q", text)
	}
	for _, want := range []string{
		"shutdown  trigger=expected_stop",
		"open_connections=",
		"urgent=false",
		"idle_closed=",
		"force_closed=",
		"aborted=",
		"stale_connections=",
		"cancelled_tasks=",
		"└◦ server.shutdown.closed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("shutdown block missing %q: %q", want, text)
		}
	}
}

func requestLines(text string) []string {
	return regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d [●×] .*$`).FindAllString(text, -1)
}

func durationSuffix(line string) time.Duration {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0
	}
	match := regexp.MustCompile(`(\d+(?:\.\d+)?)(µs|ms|s)$`).FindStringSubmatch(fields[len(fields)-1])
	if match == nil {
		return 0
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	switch match[2] {
	case "µs":
		return time.Duration(value * float64(time.Microsecond))
	case "ms":
		return time.Duration(value * float64(time.Millisecond))
	default:
		return time.Duration(value * float64(time.Second))
	}
}
