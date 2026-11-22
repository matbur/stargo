# Spec Delta

## ADDED Requirements

### Requirement: HTTP/2 streaming preset
The framework SHALL provide a server option that configures HTTP/2 for many concurrent long-lived streams, including a higher concurrent stream limit, larger receive buffers, health-check pings, and a bounded write-byte timeout.

#### Scenario: Preset applied
- **WHEN** an application serves with the HTTP/2 streaming preset
- **THEN** the underlying server has non-zero concurrent stream, receive buffer, ping, and write-byte timeout settings

#### Scenario: Individual override
- **WHEN** the generic server option runs after the preset
- **THEN** it can replace individual HTTP/2 fields without losing the rest of the preset

#### Scenario: HTTP/1.1 unaffected
- **WHEN** a client connects without HTTP/2 negotiation
- **THEN** the server still serves normal HTTP/1.1 responses
