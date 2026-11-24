# Tasks

## 1. Password hashing

- [x] 1.1 Add argon2id `HashPassword` and `VerifyPassword` with a PHC encoding, random salt, constant-time comparison, and parameter parsing; test format, salt uniqueness, correct and wrong passwords, and malformed hashes.

## 2. Session login and gate

- [x] 2.1 Add `User`, `Login`, `Logout`, `UserFromSession`, and `UserFromRequest` over the session package with JSON storage and rotation on login; test login, session reuse, logout, missing session, missing id, and malformed stored values.
- [x] 2.2 Add `RequireUser` with redirect and unauthorized options and the default 401; test authenticated, unauthenticated, redirect, and custom responses.

## 3. Example and documentation

- [x] 3.1 Add `examples/auth` with a login form, protected page, logout, and tests, and wire it into `Makefile` and the README examples list.
- [x] 3.2 Document password hashing, login/logout, the gate, and middleware order in `README.md` and package docs; update `ROADMAP.md`.

## 4. Verification

- [x] 4.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./auth ./examples/auth`; confirm all are clean.
- [x] 4.2 Run the chat-room module tests and `openspec validate --changes --strict`; confirm both pass.
