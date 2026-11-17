# Proposal

## Why

Stargo currently starts silently: no startup banner, no per-request line, and no shutdown summary. Developers coming from Stario lose the quick read on which requests ran, how long they took, and what the server configuration was. Stargo should print the same human-readable console output during development.

## What Changes

- Print a startup summary when the server begins listening: app name, listen address, shutdown timeout, and request timeouts.
- Print one line per finished request with the start time, method, path, status, and duration. Unmatched paths (404, 405) use the same line.
- Print a shutdown summary with the trigger, connection counters, and a nested `server.shutdown.closed` event.
- Format output like Stario's TTY renderer: clock, `●`, name, status, right-aligned duration, indented attributes with folded dotted prefixes, and `│`/`├`/`└◦` nesting for shutdown events.
- Enable console output automatically when standard output is a terminal, disable it when redirected, and allow explicit control through an environment variable and a serve option.
- Keep NDJSON tracing unchanged and independent, so `TRACE` still writes spans while console output is on.

## Capabilities

### New Capabilities
- `console-logging`: startup banner, per-request access lines, shutdown summary, formatting rules, and enable/disable configuration.

### Modified Capabilities

## Impact

- New console renderer in the root package or a small internal package.
- `server.go`: startup and shutdown reporting, connection accounting through `http.Server.ConnState`.
- `app.go`: timing and status capture around request handling, including unmatched requests.
- `README.md`: document console output and its environment variable.
- No change to the handler API, response helpers, or trace export format.
