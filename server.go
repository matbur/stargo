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
)

// ServeOption configures the HTTP server started by Serve or ListenAndServe.
type ServeOption func(*serveConfig)

type serveConfig struct {
	shutdownTimeout   time.Duration
	readHeaderTimeout time.Duration
	idleTimeout       time.Duration
	configure         func(*http.Server)
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
	if cfg.configure != nil {
		cfg.configure(server)
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
		return err
	case <-runCtx.Done():
		// Cancel request contexts first: Shutdown waits for handlers but does
		// not cancel them, and a live stream would wait forever.
		a.cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
		defer shutdownCancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = server.Close()
			<-errCh
			return shutdownErr
		}

		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
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
