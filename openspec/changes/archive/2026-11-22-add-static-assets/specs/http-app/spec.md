# Spec Delta

## ADDED Requirements

### Requirement: Standard handler mounting
The application SHALL register a standard `http.Handler` on a raw pattern while keeping the request span, route attributes, incoming trace context, and status recording that `Ctx` handlers receive.

#### Scenario: Mounted handler
- **WHEN** an application mounts an `http.Handler` on `/assets/`
- **THEN** the handler writes the response and the request span carries the pattern, method, path, and status

#### Scenario: Existing trace
- **WHEN** a request with a valid `traceparent` reaches a mounted handler
- **THEN** the request span continues that trace

#### Scenario: Status recorded
- **WHEN** a mounted handler answers 404
- **THEN** the span status attribute is 404
