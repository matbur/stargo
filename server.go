package stargo

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matbur/stargo/internal/console"
)

// ServeOption configures the HTTP server started by Serve or ListenAndServe.
type ServeOption func(*serveConfig)

type serveConfig struct {
	shutdownTimeout   time.Duration
	readHeaderTimeout time.Duration
	idleTimeout       time.Duration
	http2             *http.HTTP2Config
	configure         func(*http.Server)
	console           *consoleTarget
}

func defaultServeConfig() serveConfig {
	return serveConfig{
		shutdownTimeout:   10 * time.Second,
		readHeaderTimeout: 5 * time.Second,
		idleTimeout:       120 * time.Second,
	}
}

// ShutdownTimeout bounds how long shutdown waits for in-flight handlers.
func ShutdownTimeout(timeout time.Duration) ServeOption {
	return func(c *serveConfig) {
		if timeout > 0 {
			c.shutdownTimeout = timeout
		}
	}
}

// ReadHeaderTimeout bounds how long the server waits for request headers.
// Zero disables the timeout.
func ReadHeaderTimeout(timeout time.Duration) ServeOption {
	return func(c *serveConfig) {
		if timeout >= 0 {
			c.readHeaderTimeout = timeout
		}
	}
}

// Server exposes the underlying *http.Server for settings without a dedicated
// option.
func Server(configure func(*http.Server)) ServeOption {
	return func(c *serveConfig) {
		c.configure = configure
	}
}

// HTTP2Streaming returns a preset tuned for many concurrent long-lived
// streams: more concurrent streams, larger receive buffers, health-check
// pings, and a bounded write-byte timeout. It only affects HTTP/2 connections.
//
// The preset is applied before the Server callback, so an application can
// override individual fields.
func HTTP2Streaming() ServeOption {
	return func(c *serveConfig) {
		c.http2 = &http.HTTP2Config{
			MaxConcurrentStreams:          250,
			MaxReadFrameSize:              32 << 10,
			MaxReceiveBufferPerConnection: 1 << 20,
			MaxReceiveBufferPerStream:     256 << 10,
			SendPingTimeout:               30 * time.Second,
			PingTimeout:                   15 * time.Second,
			WriteByteTimeout:              time.Minute,
		}
	}
}

// Serve serves HTTP on ln until ctx is cancelled, the application is closed,
// or the server fails.
//
// Request contexts are cancelled before the server stops accepting
// connections, handlers get shutdownTimeout to finish, and the tracer is
// drained before Serve returns. Signal handling is not installed; use
// ListenAndServe for that.
func (a *App) Serve(ctx context.Context, ln net.Listener, opts ...ServeOption) error {
	if ln == nil {
		return errors.New("stargo: listener is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := defaultServeConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	startupStart := time.Now()
	writer := resolveConsole(cfg.console)
	a.console.Store(writer)
	defer a.console.Store(nil)

	// Shut down when either the caller's context or the application root is
	// cancelled.
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	stopRootWatch := context.AfterFunc(a.rootCtx, cancelRun)
	defer stopRootWatch()

	server := &http.Server{
		Handler:           a,
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		WriteTimeout:      0, // streamed responses need an unbounded write time
		IdleTimeout:       cfg.idleTimeout,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		BaseContext: func(net.Listener) context.Context {
			return a.rootCtx
		},
	}
	if cfg.http2 != nil {
		server.HTTP2 = cfg.http2
	}
	if cfg.configure != nil {
		cfg.configure(server)
	}

	stats := newConnStats()
	userConnState := server.ConnState
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		stats.observe(conn, state)
		if userConnState != nil {
			userConnState(conn, state)
		}
	}

	if writer != nil {
		startupCfg := cfg
		startupCfg.readHeaderTimeout = server.ReadHeaderTimeout
		startupCfg.idleTimeout = server.IdleTimeout
		writer.Write(console.Block{
			Start:    startupStart,
			Name:     "server.startup",
			Duration: time.Since(startupStart),
			Attrs:    startupAttrs(a, ln, startupCfg),
		})
	}

	defer a.tracer.Close()
	startup := a.tracer.Start("startup", "startup")
	startup.Attr("server.addr", ln.Addr().String())
	defer startup.End()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		if writer != nil {
			open := stats.openCount()
			writer.Write(shutdownBlock("runtime_failure", time.Now(), stats.snapshot(open, 0, 0), err))
		}
		return err
	case <-runCtx.Done():
		shutdownStart := time.Now()
		open := stats.openCount()
		cancelled := int(a.inflight.Load())
		stats.beginShutdown()

		// Cancel request contexts first: Shutdown waits for handlers but does
		// not cancel them, and a live stream would wait forever.
		a.cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
		defer shutdownCancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		aborted := 0
		if shutdownErr != nil {
			aborted = stats.openCount()
			_ = server.Close()
		}
		serveErr := <-errCh

		trigger := "expected_stop"
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			trigger = "runtime_failure"
		}
		if writer != nil {
			counts := stats.snapshot(open, cancelled, aborted)
			writer.Write(shutdownBlock(trigger, shutdownStart, counts, shutdownErr))
		}

		if shutdownErr != nil {
			return shutdownErr
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
}

// ListenAndServe listens on addr and serves until an interrupt or termination
// signal arrives.
func (a *App) ListenAndServe(addr string, opts ...ServeOption) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.listenAndServe(ctx, addr, opts...)
}

func (a *App) listenAndServe(ctx context.Context, addr string, opts ...ServeOption) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return a.Serve(ctx, listener, opts...)
}
