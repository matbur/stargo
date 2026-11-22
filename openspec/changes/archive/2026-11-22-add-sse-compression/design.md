# Design

## Compression is opt-in through the request

`Stream` gains a `WithRequest(*http.Request)` option. It copies the
`Accept-Encoding` decision and leaves the context alone, so callers may keep
using `WithContext`. `Ctx.Stream` passes the request automatically, which makes
browser SSE connections benefit without a new application option. Direct
`sse.New` callers opt in by passing the request.

Negotiation parses comma-separated encodings with optional `q` values,
case-insensitively. The server preference list currently contains only `gzip`,
because the standard library provides it and the module keeps its single
dependency. A wildcard accepts gzip when gzip itself is not excluded.
`gzip;q=0` disables compression even when a wildcard is present.

## Gzip writer lifecycle

The gzip writer is created on the first event, after headers are decided. Each
event is written through it, then the gzip writer is flushed, then the
`http.ResponseController` flushes the connection. This preserves the existing
"one event, one write, one flush" contract from the client's point of view.

`Close` closes the gzip trailer only when the stream started. Closing an
unstarted stream must not emit gzip bytes before HTTP headers.

## Safety decisions

Compression is skipped when:

- the response already carries a `Content-Encoding`;
- the application set `Cache-Control: no-transform`;
- the request method is `HEAD`.

`Content-Type: text/event-stream` and `Cache-Control: no-cache` are set exactly
as before. `Vary: Accept-Encoding` is added when compression is selected so
caches keep encoded and identity responses apart. `X-Accel-Buffering: no`
remains, and the README documents `no-transform` and proxy buffering.

No size threshold is applied: SSE headers are sent before the payload is
complete, so a stream-level threshold would delay the first event and a
per-event threshold would require switching encodings mid-response.

## HTTP/2 preset

Go 1.27 exposes `http.Server.HTTP2 *http.HTTP2Config`. `HTTP2Streaming()`
returns a `ServeOption` that sets stream-oriented values: more concurrent
streams, larger per-stream and per-connection receive buffers, a read frame
size suitable for small patches, health-check pings, and a bounded write-byte
timeout that still allows idle streams to stay open.

`Serve` assigns the preset before calling the generic `stargo.Server`
configure callback, so an application can override individual fields. The
preset only affects HTTP/2 connections; plaintext HTTP/1.1 and h2c are not
started by it.

## Risks

- Compression costs CPU per stream. The default negotiation only turns it on
  when the client asks, and `no-transform` or a stripped `Accept-Encoding`
  header disables it.
- A proxy that ignores `X-Accel-Buffering` can still buffer compressed events;
  documentation names the setting and the disable switch.
- gzip has no dictionary reset. The stream keeps one gzip stream for its whole
  lifetime; resetting would be a protocol-visible change and is avoided.
