# Spec Delta

## MODIFIED Requirements

### Requirement: Stream compression
The stream SHALL compress events with brotli or gzip when the request advertises an encoding through `Accept-Encoding`, honoring q-values, excluding an explicitly refused encoding, and preferring brotli when qualities tie.

#### Scenario: Gzip accepted
- **WHEN** a stream is created from a request with `Accept-Encoding: gzip`
- **THEN** events are gzip-compressed and the response carries `Content-Encoding: gzip`

#### Scenario: Brotli accepted
- **WHEN** the request advertises `br`
- **THEN** events are brotli-compressed and the response carries `Content-Encoding: br`

#### Scenario: Server preference
- **WHEN** the request advertises `gzip` and `br` with equal quality
- **THEN** the stream selects brotli

#### Scenario: Quality wins
- **WHEN** the request advertises `br;q=0.2, gzip;q=0.9`
- **THEN** the stream selects gzip

#### Scenario: Gzip refused
- **WHEN** the request contains `gzip;q=0`
- **THEN** events are written without compression and no `Content-Encoding` header is set

#### Scenario: Encoding refused
- **WHEN** the request contains `gzip;q=0` and no other accepted encoding
- **THEN** events are written without compression and no `Content-Encoding` header is set

#### Scenario: No acceptable encoding
- **WHEN** the request has no `Accept-Encoding` header
- **THEN** the stream stays uncompressed and keeps the existing headers

#### Scenario: Cache separation
- **WHEN** the stream selects a compression encoding
- **THEN** the response carries `Vary: Accept-Encoding`

### Requirement: Compressed event delivery
When compression is active, each event SHALL be written through the selected compressor, the compressor SHALL be flushed, and the connection SHALL be flushed before the call returns.

#### Scenario: Per-event flush
- **WHEN** an event is written on a compressed stream
- **THEN** the client can decode it without waiting for the next event

#### Scenario: Decodable stream
- **WHEN** a compressed stream is closed
- **THEN** its body is a valid stream in the selected encoding containing every written event in order
