# Spec Delta

## Purpose

Turns a verified login into an authenticated request with argon2id password hashing, a session-backed user, and a middleware gate.

## ADDED Requirements

### Requirement: Password hashing
The framework SHALL hash passwords with argon2id, a random per-hash salt, and parameters encoded in a PHC string, and SHALL verify them in constant time.

#### Scenario: Hash format
- **WHEN** a password is hashed
- **THEN** the result starts with `$argon2id$` and carries the version, memory, iterations, parallelism, salt, and key

#### Scenario: Unique salt
- **WHEN** the same password is hashed twice
- **THEN** the two hashes differ

#### Scenario: Correct password
- **WHEN** the correct password is verified against a hash
- **THEN** verification returns true and no error

#### Scenario: Wrong password
- **WHEN** another password is verified against the hash
- **THEN** verification returns false and no error

#### Scenario: Malformed hash
- **WHEN** verification receives a string that is not a valid argon2id PHC hash
- **THEN** it returns an error

### Requirement: Session login and logout
Login SHALL rotate the session id and store the user, and logout SHALL destroy the session.

#### Scenario: Login
- **WHEN** a handler logs in a user with a session
- **THEN** the previous id no longer loads, the new session carries the user, and a later request sees the same user

#### Scenario: Missing session
- **WHEN** login is called without a session
- **THEN** it returns an error and does not store a user

#### Scenario: Missing user id
- **WHEN** login is called with an empty user id
- **THEN** it returns an error

#### Scenario: Logout
- **WHEN** a handler logs out
- **THEN** the session is destroyed and later requests are unauthenticated

#### Scenario: Malformed stored user
- **WHEN** the stored session value is not a valid user
- **THEN** the request is treated as unauthenticated

### Requirement: RequireUser middleware
The framework SHALL provide middleware that allows a request only when its session carries a user, and otherwise redirects or invokes a configurable unauthorized response.

#### Scenario: Authenticated request
- **WHEN** the session carries a user
- **THEN** the wrapped handler runs

#### Scenario: Unauthenticated request
- **WHEN** the session carries no user and no redirect is configured
- **THEN** the middleware answers 401 without calling the wrapped handler

#### Scenario: Redirect
- **WHEN** a redirect path is configured and the session carries no user
- **THEN** the middleware answers 303 with a `Location` header and does not call the wrapped handler

#### Scenario: Custom unauthorized response
- **WHEN** an unauthorized handler is configured and no redirect is set
- **THEN** the middleware calls that handler instead of the wrapped handler
