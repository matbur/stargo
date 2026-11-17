package stargo

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matbur/stargo/internal/console"
)

// consoleTarget is the console destination chosen by serve options.
type consoleTarget struct {
	writer io.Writer
	set    bool
}

// Console writes human-readable startup, request, and shutdown blocks to w.
//
// It overrides STARGO_CONSOLE. Passing nil is equivalent to NoConsole.
func Console(w io.Writer) ServeOption {
	return func(c *serveConfig) {
		c.console = &consoleTarget{writer: w, set: true}
	}
}

// NoConsole disables human-readable console output.
func NoConsole() ServeOption {
	return func(c *serveConfig) {
		c.console = &consoleTarget{set: true}
	}
}

// resolveConsole applies the serve option, the environment, and terminal
// detection. It returns nil when console output is disabled.
func resolveConsole(target *consoleTarget) *console.Writer {
	writer := resolvedWriter(target)
	if writer == nil {
		return nil
	}
	return console.NewWriter(writer, consoleWidth(), colorEnabled(writer))
}

func resolvedWriter(target *consoleTarget) io.Writer {
	if target != nil && target.set {
		return target.writer
	}
	switch consoleEnv() {
	case "off":
		return nil
	case "on":
		return os.Stdout
	default:
		if !isTerminal(os.Stdout) {
			return nil
		}
		return os.Stdout
	}
}

// consoleEnv maps STARGO_CONSOLE to "on", "off", or "auto".
func consoleEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STARGO_CONSOLE"))) {
	case "0", "off", "false":
		return "off"
	case "1", "true", "on":
		return "on"
	default:
		return "auto"
	}
}

// consoleWidth returns one cell less than COLUMNS, capped at 100.
func consoleWidth() int {
	columns, err := strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS")))
	if err != nil || columns <= 0 {
		return 100
	}
	width := columns - 1
	if width > 100 {
		width = 100
	}
	if width < 1 {
		width = 1
	}
	return width
}

func colorEnabled(writer io.Writer) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	return isTerminal(writer)
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// displayName returns the configured application name or the executable base
// name.
func (a *App) displayName() string {
	if a.name != "" {
		return a.name
	}
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "stargo"
	}
	return name
}

// startupAttrs describes the configuration actually used to serve.
func startupAttrs(app *App, listener net.Listener, cfg serveConfig) []console.Attr {
	attrs := []console.Attr{{Key: "app.name", Value: app.displayName()}}
	if listener.Addr().Network() == "unix" {
		attrs = append(attrs,
			console.Attr{Key: "server.listen_mode", Value: "unix_socket"},
			console.Attr{Key: "server.unix_socket", Value: listener.Addr().String()},
		)
	} else {
		host, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			host = listener.Addr().String()
			port = ""
		}
		attrs = append(attrs,
			console.Attr{Key: "server.listen_mode", Value: "tcp"},
			console.Attr{Key: "server.host", Value: host},
			console.Attr{Key: "server.port", Value: port},
		)
	}
	attrs = append(attrs,
		console.Attr{Key: "server.tls", Value: listenerIsTLS(listener)},
		console.Attr{Key: "server.listening", Value: true},
		console.Attr{Key: "server.graceful_shutdown_timeout", Value: cfg.shutdownTimeout},
		console.Attr{Key: "server.timeout.request_header", Value: cfg.readHeaderTimeout},
		console.Attr{Key: "server.timeout.keep_alive", Value: cfg.idleTimeout},
	)
	return attrs
}

// listenerIsTLS reports whether the listener came from crypto/tls. The TLS
// listener type is unexported, so its type name is the only signal available.
func listenerIsTLS(listener net.Listener) bool {
	return strings.Contains(fmt.Sprintf("%T", listener), "tls")
}

// shutdownBlock builds the shutdown summary. The failure is reported only when
// shutdown or serving failed.
func shutdownBlock(trigger string, start time.Time, counts shutdownCounts, failure error) console.Block {
	elapsed := time.Since(start)
	block := console.Block{
		Start:    start,
		Name:     "server.shutdown",
		Duration: elapsed,
		Attrs: []console.Attr{
			{Key: "server.shutdown.trigger", Value: trigger},
			{Key: "server.shutdown.open_connections", Value: counts.Open},
			{Key: "server.shutdown.urgent", Value: false},
			{Key: "server.shutdown.idle_closed", Value: counts.IdleClosed},
			{Key: "server.shutdown.force_closed", Value: counts.ForceClosed},
			{Key: "server.shutdown.aborted", Value: counts.Aborted},
			{Key: "server.shutdown.stale_connections", Value: counts.Stale},
			{Key: "server.shutdown.cancelled_tasks", Value: counts.Cancelled},
		},
		Event: &console.Event{Name: "server.shutdown.closed", Offset: elapsed},
	}
	if failure != nil {
		block.Failed = true
		block.Attrs = append(block.Attrs, console.Attr{Key: "error", Value: failure.Error()})
	}
	return block
}
