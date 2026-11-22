# Design

## Negotiation

`Accept-Encoding` is parsed once into a map of encoding name to quality. The
supported list is `br`, then `gzip`; a name absent from the header uses the
wildcard quality when present, and an explicit `q=0` excludes that name even
when a wildcard allows it. The highest quality above zero wins, and a tie
follows the server preference. `x-gzip` remains an alias for gzip.

## Compressor seam

Both `gzip.Writer` and `brotli.Writer` satisfy the same small interface: write,
flush, close. The stream keeps the selected encoding and one compressor. Each
event writes through it, flushes it, then flushes the connection. `Close`
closes the compressor once, and only after the stream started.

The existing safety rules stay shared: an application-set `Content-Encoding`,
`Cache-Control: no-transform`, and `HEAD` all skip compression.

## Dependency

`github.com/andybalholm/brotli` is pure Go, has no transitive dependencies, and
is the reference implementation used by most Go servers. It becomes the second
direct dependency. The README's dependency statement is updated.

## Risks

- Brotli costs more CPU than gzip. Negotiation only selects it when the client
  asks, and `no-transform` or a stripped `Accept-Encoding` disables
  compression entirely.
- Quality is left at the library default. A future option could expose it
  without changing the wire contract.
