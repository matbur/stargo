# Spec Delta

## REMOVED Requirements

### Requirement: No stream compression
**Reason**: Streams may now be compressed when the client negotiates gzip, so the unconditional prohibition is no longer true.
**Migration**: No caller action is required. Applications that depend on identity encoding can set `Cache-Control: no-transform` or strip the `Accept-Encoding` header before opening a stream.

## ADDED Requirements

### Requirement: Stream compression
The stream SHALL compress events with gzip when the request advertises gzip through `Accept-Encoding`, honoring q-values and excluding gzip when it is explicitly refused.

#### Scenario: Gzip accepted
- **WHEN** a stream is created from a request with `Accept-Encoding: gzip`
- **THEN** events are gzip-compressed and the response carries `Content-Encoding: gzip`

#### Scenario: Gzip refused
- **WHEN** the request contains `gzip;q=0`
- **THEN** events are written without compression and no `Content-Encoding` header is set

#### Scenario: No acceptable encoding
- **WHEN** the request has no `Accept-Encoding` header
- **THEN** the stream stays uncompressed and keeps the existing headers

#### Scenario: Cache separation
- **WHEN** the stream selects gzip compression
- **THEN** the response carries `Vary: Accept-Encoding`

### Requirement: Compressed event delivery
When compression is active, each event SHALL be written through the gzip writer, the gzip writer SHALL be flushed, and the connection SHALL be flushed before the call returns.

#### Scenario: Per-event flush
- **WHEN** an event is written on a compressed stream
- **THEN** the client can decode it without waiting for the next event

#### Scenario: Decodable stream
- **WHEN** a compressed stream is closed
- **THEN** its body is a valid gzip stream containing every written event in order

### Requirement: Compression safety
The stream SHALL NOT add compression when the response already has a `Content-Encoding`, when `Cache-Control` contains `no-transform`, or when the request method is `HEAD`, and SHALL keep the event-stream content type and cache headers unchanged.

#### Scenario: Existing encoding
- **WHEN** the response already carries a `Content-Encoding` header
- **THEN** the stream writes identity bytes and does not replace that header

#### Scenario: No transform
- **WHEN** the application sets `Cache-Control: no-transform` before the first event
- **THEN** the stream stays uncompressed

#### Scenario: Head request
- **WHEN** the request method is `HEAD`
- **THEN** the stream does not select compression

#### Scenario: Headers preserved
- **WHEN** a compressed stream sends its first event
- **THEN** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and `X-Accel-Buffering: no` are still present
