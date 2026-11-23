# sessions Specification

## Purpose

Keeps server-side session state across requests using an opaque cookie-backed id, a pluggable store, and middleware that loads and commits the session.

## Requirements

### Requirement: Session identity and cookie
The session SHALL use an opaque random id stored in a cookie, and the default cookie SHALL be named `__Host-session`, use `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`, and no `Domain`.

#### Scenario: First use
- **WHEN** a handler modifies a session for a request without a valid session cookie
- **THEN** the response sets the session cookie with a new random id and the stored record uses that id

#### Scenario: Unused session
- **WHEN** a handler never reads or modifies the session
- **THEN** the response sets no session cookie and the store receives no record

#### Scenario: Cookie reload
- **WHEN** a request carries the session cookie
- **THEN** the middleware loads the matching record and exposes it to the handler

#### Scenario: Local development
- **WHEN** the manager is created with `Insecure` and the default cookie name
- **THEN** the cookie is named `session` and does not require `Secure`

### Requirement: Session store contract
The framework SHALL define a store contract that loads, saves, and deletes a record containing values, a creation time, and an expiry time, and SHALL provide an in-memory implementation.

#### Scenario: Save and load
- **WHEN** a record is saved and loaded again before it expires
- **THEN** the loaded values and timestamps match the saved record

#### Scenario: Delete
- **WHEN** a record is deleted
- **THEN** a later load reports no record

#### Scenario: Expired record
- **WHEN** a record is loaded after its expiry
- **THEN** the load reports no record and the memory store removes it

#### Scenario: Sweep
- **WHEN** the memory store sweeps expired records
- **THEN** expired records are removed and live records remain

### Requirement: Session data access
A session SHALL expose key-based reads, writes, and deletion over a per-session data map, and a typed lookup helper SHALL return a value of the requested type.

#### Scenario: Set and get
- **WHEN** a handler stores a value and reads the same key
- **THEN** it receives the stored value and the key reports presence

#### Scenario: Typed lookup
- **WHEN** a stored value has the requested type
- **THEN** the generic lookup returns it and true; a different type returns false

#### Scenario: Delete key
- **WHEN** a handler deletes a key
- **THEN** later reads report absence

### Requirement: Session lifecycle
The middleware SHALL save a used session after the handler, SHALL rotate the id on `Rotate` while keeping the data and deleting the previous record, and SHALL delete the record and clear the cookie on `Destroy`.

#### Scenario: Save after handler
- **WHEN** a handler modifies a loaded session
- **THEN** the record is saved with the new values before the response handling ends

#### Scenario: Rotation
- **WHEN** a handler calls `Rotate`
- **THEN** the response sets a new id, the previous id no longer loads, and the data is available under the new id

#### Scenario: Destruction
- **WHEN** a handler calls `Destroy`
- **THEN** the response clears the cookie and the record no longer loads

#### Scenario: Unknown cookie
- **WHEN** the cookie contains an id the store does not know
- **THEN** the request starts a fresh session

### Requirement: Session expiration
Sessions SHALL expire after an idle timeout that slides on use and after an absolute lifetime anchored at creation.

#### Scenario: Idle expiry
- **WHEN** a session is not used for longer than the idle timeout
- **THEN** the next request starts a fresh session

#### Scenario: Sliding use
- **WHEN** a session is used again before the idle timeout
- **THEN** its idle deadline moves forward

#### Scenario: Absolute expiry
- **WHEN** a session reaches its absolute lifetime even though it is used regularly
- **THEN** the next request starts a fresh session

#### Scenario: Disabled absolute limit
- **WHEN** the absolute lifetime is zero
- **THEN** only the idle timeout applies

### Requirement: Session error handling
The middleware SHALL return an error response when the store cannot load a session, and SHALL report a commit failure through a configurable error callback because the response may already be written.

#### Scenario: Load failure
- **WHEN** the store fails to load an existing session
- **THEN** the middleware responds 500 and does not call the handler

#### Scenario: Save failure
- **WHEN** the store fails to save after the handler
- **THEN** the configured error callback receives the error and the response is unchanged
