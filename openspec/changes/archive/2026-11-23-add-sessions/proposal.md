## Why

Applications can write cookies but cannot keep server-side state across
requests. Every login, cart, or chat identity has to be recreated from request
parameters, and there is no way to log a user out or rotate an identifier. This
is the first half of roadmap item 5.4; authentication and CSRF stay separate.

## What Changes

- Add a `session` package with a cookie-backed random session id and a
  per-session data map.
- Define a small `Store` contract so a database or Redis store can replace the
  in-memory one.
- Provide `MemoryStore` with lazy expiration and an explicit `Sweep`.
- Provide a `Manager` that loads the session from the cookie, attaches it to
  the request context, and saves or destroys it after the handler.
- Support sliding idle expiration and an absolute lifetime, `Rotate`, and
  `Destroy`.
- Default to a `__Host-session` cookie with `Secure`, `HttpOnly`, and
  `SameSite=Lax`; add an `Insecure` escape for local HTTP development.
- Update README, package docs, and roadmap.

## Capabilities

### New Capabilities

- `sessions`: cookie-backed server-side sessions with a store contract, idle
  and absolute expiration, rotation, destruction, and request middleware.

## Impact

- New `session` package and tests.
- `README.md`, `ROADMAP.md`.
- No new module dependencies; the package reuses `cookies`.
