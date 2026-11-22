package stargo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/trace"
)

func testApp(t *testing.T) *App {
	t.Helper()
	app := New(WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
}

func waitFor(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(message)
}

func getAsync(url string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(url)
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()
	return done
}

func TestServeCancelsBlockedHandler(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	entered := make(chan struct{})
	returned := make(chan struct{})
	app.Handle(route.Get("/"), func(c *Ctx) error {
		close(entered)
		<-c.R.Context().Done()
		close(returned)
		return nil
	})

	listener := listen(t)
	defer listener.Close()
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errCh <- app.Serve(ctx, listener) }()

	_ = getAsync("http://" + listener.Addr().String() + "/")

	<-entered
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve() did not return after cancellation")
	}

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("handler did not observe context cancellation")
	}
}

func TestServeOptionsReachServer(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	app.Handle(route.Get("/"), func(c *Ctx) error { return c.NoContent() })

	configured := make(chan *http.Server, 1)
	listener := listen(t)
	defer listener.Close()
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errCh <- app.Serve(ctx, listener,
			ReadHeaderTimeout(2*time.Second),
			HTTP2Streaming(),
			Server(func(server *http.Server) {
				if server.HTTP2 != nil {
					server.HTTP2.MaxConcurrentStreams = 99
				}
				configured <- server
			}),
		)
	}()

	var captured *http.Server
	select {
	case captured = <-configured:
	case <-time.After(time.Second):
		t.Fatal("Server option did not reach the server")
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if captured.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v", captured.ReadHeaderTimeout)
	}
	if captured.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0", captured.WriteTimeout)
	}
	if captured.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %v", captured.IdleTimeout)
	}
	if captured.BaseContext == nil {
		t.Fatal("BaseContext is nil")
	}
	if captured.HTTP2 == nil {
		t.Fatal("HTTP2 preset was not applied")
	}
	if captured.HTTP2.MaxConcurrentStreams != 99 {
		t.Fatalf("MaxConcurrentStreams = %d, want 99", captured.HTTP2.MaxConcurrentStreams)
	}
	if captured.HTTP2.MaxReceiveBufferPerConnection != 1<<20 ||
		captured.HTTP2.MaxReceiveBufferPerStream != 256<<10 {
		t.Fatalf("receive buffers = %d/%d",
			captured.HTTP2.MaxReceiveBufferPerConnection,
			captured.HTTP2.MaxReceiveBufferPerStream)
	}
	if captured.HTTP2.SendPingTimeout != 30*time.Second ||
		captured.HTTP2.PingTimeout != 15*time.Second ||
		captured.HTTP2.WriteByteTimeout != time.Minute {
		t.Fatalf("HTTP2 timeouts = %v/%v/%v",
			captured.HTTP2.SendPingTimeout,
			captured.HTTP2.PingTimeout,
			captured.HTTP2.WriteByteTimeout)
	}
}

func TestShutdownTimeoutReturnsDeadline(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	entered := make(chan struct{})
	block := make(chan struct{})
	app.Handle(route.Get("/"), func(c *Ctx) error {
		close(entered)
		<-block
		return nil
	})

	listener := listen(t)
	defer listener.Close()
	buffer := &lockedBuffer{}
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errCh <- app.Serve(ctx, listener, ShutdownTimeout(50*time.Millisecond), Console(buffer))
	}()

	_ = getAsync("http://" + listener.Addr().String() + "/")
	<-entered
	start := time.Now()
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Serve() error = %v, want context.DeadlineExceeded", err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("shutdown took %v", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve() hung after the shutdown timeout")
	}
	close(block)

	match := regexp.MustCompile(`force_closed=(\d+)`).FindStringSubmatch(buffer.String())
	if match == nil {
		t.Fatalf("shutdown block missing force_closed: %q", buffer.String())
	}
	forceClosed, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("force_closed value: %v", err)
	}
	if forceClosed == 0 {
		t.Fatalf("force_closed = 0 after a shutdown timeout: %q", buffer.String())
	}
}

func TestWriteSurvivesReadHeaderTimeout(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	app.Handle(route.Get("/"), func(c *Ctx) error {
		time.Sleep(60 * time.Millisecond)
		return c.Text("late")
	})

	listener := listen(t)
	defer listener.Close()
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		errCh <- app.Serve(ctx, listener, ReadHeaderTimeout(20*time.Millisecond))
	}()

	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "late" {
		t.Fatalf("body = %q, want late", body)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	listener := listen(t)
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func TestListenAndServeCancelsOpenStream(t *testing.T) {
	app := testApp(t)
	entered := make(chan struct{})
	returned := make(chan struct{})
	app.Handle(route.Get("/live"), func(c *Ctx) error {
		stream, err := c.Stream()
		if err != nil {
			return err
		}
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			return err
		}
		close(entered)
		<-c.R.Context().Done()
		close(returned)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	address := freeAddr(t)
	go func() { errCh <- app.listenAndServe(ctx, address) }()

	_ = getAsync("http://" + address + "/live")

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("stream handler did not start")
	}

	start := time.Now()
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("listenAndServe() error = %v", err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("shutdown took %v", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listenAndServe() did not return")
	}

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("handler did not return")
	}
}

func TestTraceDrainedBeforeServeReturns(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	tracer := trace.New(
		trace.Destination(buffer),
		trace.BatchSize(1),
		trace.FlushInterval(time.Millisecond),
	)
	app := New(WithTracer(tracer))
	t.Cleanup(func() { _ = app.Close() })
	app.Handle(route.Get("/"), func(c *Ctx) error { return c.Text("ok") })

	listener := listen(t)
	defer listener.Close()
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errCh <- app.Serve(ctx, listener) }()

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

	names := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(buffer.Bytes()))
	for scanner.Scan() {
		var record struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid span %q: %v", scanner.Text(), err)
		}
		names[record.Name] = true
		if record.Name == "startup" && record.Kind != "startup" {
			t.Fatalf("startup kind = %q", record.Kind)
		}
	}
	if !names["GET /"] {
		t.Fatalf("request span missing from %q", buffer.String())
	}
	if !names["startup"] {
		t.Fatalf("startup span missing from %q", buffer.String())
	}
}

func TestListenAndServeHandlesSignal(t *testing.T) {
	if testing.Short() {
		t.Skip("sends a process signal")
	}
	app := testApp(t)
	app.Handle(route.Get("/"), func(c *Ctx) error { return c.NoContent() })

	address := freeAddr(t)
	errCh := make(chan error, 1)
	go func() { errCh <- app.ListenAndServe(address) }()

	waitFor(t, func() bool {
		client := http.Client{Timeout: 200 * time.Millisecond}
		response, err := client.Get("http://" + address + "/")
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return true
	}, 3*time.Second, "server did not start")

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ListenAndServe() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ListenAndServe() did not return after a signal")
	}
}

func TestServeReturnsListenerError(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	listener := listen(t)
	_ = listener.Close() // Serve on a closed listener fails immediately

	err := app.Serve(context.Background(), listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve() error = %v, want a listener failure", err)
	}
}

func TestListenAndServeReturnsBindError(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	listener := listen(t)
	defer listener.Close()
	err := app.listenAndServe(context.Background(), listener.Addr().String())
	if err == nil {
		t.Fatal("expected a bind error")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("error = %v", err)
	}
}
