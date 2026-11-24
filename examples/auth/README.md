# Auth

Password login with a server-side session, a protected page, and logout. The
demo account is `demo` with the password from `AUTH_PASSWORD` (default
`stargo-demo`).

The app registers the session middleware first, then `auth.RequireUser` with a
redirect to `/login`. The login and logout handlers use `forms`, `auth`, and
`session` directly; there is no user database, registration, or CSRF token.

## Run

From the repository root:

```sh
go run ./examples/auth
```

Open <http://localhost:8080>. The root redirects to the login page until the
session carries a user. Stop the app with `Ctrl+C`.

Set another password for the demo account:

```sh
AUTH_PASSWORD='change-me' go run ./examples/auth
```
