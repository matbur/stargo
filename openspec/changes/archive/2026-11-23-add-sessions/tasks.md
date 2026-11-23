# Tasks

## 1. Store and session model

- [x] 1.1 Add the `session` package with `Record`, the `Store` contract, and `MemoryStore` that copies records, expires on access, and exposes `Sweep`; test save/load/delete, expiration, and sweep.
- [x] 1.2 Add `Session` with id, key-based data access, typed lookup, `Rotate`, and `Destroy`; test data operations, rotation state, and destruction state.

## 2. Manager and middleware

- [x] 2.1 Add `Manager` options and validation for cookie attributes, idle and absolute timeouts, clock, and error callback; test defaults, `Insecure`, and invalid options.
- [x] 2.2 Add middleware that loads or creates the session, attaches it to the request context, writes the cookie on first modification, commits saves, deletes rotated and destroyed ids, and responds 500 on load failure; test first use, reuse, unused requests, rotation, destruction, expiry, unknown ids, and store failures.

## 3. Documentation

- [x] 3.1 Document the store contract, cookie defaults, lifecycle, expiration, and error handling in `README.md`, add package docs, and update `ROADMAP.md` for the session half of 5.4.

## 4. Verification

- [x] 4.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./session`; confirm all are clean.
- [x] 4.2 Run `openspec validate --changes --strict`; confirm the change passes.
