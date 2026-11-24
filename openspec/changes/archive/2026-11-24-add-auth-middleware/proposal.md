## Why

`session` keeps state across requests, but there is no ready-made way to turn a
verified login into an authenticated request. Applications would reimplement
session rotation, a password hash format, and the unauthenticated response each
time. This completes the authentication half of roadmap item 5.4.

## What Changes

- Add an `auth` package with argon2id password hashing and constant-time
  verification.
- Store the authenticated user in the session as JSON, rotate the session id on
  login, and destroy it on logout.
- Add `RequireUser` middleware that either redirects to a login page or answers
  with a configurable unauthorized response.
- Add an `examples/auth` application with a login form, a protected page, and
  logout; wire it into the README and Makefile.
- Add `golang.org/x/crypto` for argon2id.
- Update README, package docs, and roadmap.

## Capabilities

### New Capabilities

- `authentication`: argon2id password hashing, session-backed login and logout,
  and a `RequireUser` gate with a configurable unauthenticated response.

## Impact

- New `auth` package and tests.
- New `examples/auth` module-less example and tests.
- `go.mod`, `go.sum` for `golang.org/x/crypto`.
- `README.md`, `ROADMAP.md`, `Makefile`.
- No CSRF, registration, user database, or flash messages in this change.
