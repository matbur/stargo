// Package auth turns a verified login into an authenticated request.
//
// Passwords are hashed with argon2id and verified in constant time. Login
// stores a user in the session and rotates its id; Logout destroys the session.
// RequireUser guards handlers that need a user:
//
//	app.Use(sessionMiddleware)
//	app.Use(auth.RequireUser(auth.RedirectTo("/login")))
//
// The session middleware must run first, because RequireUser reads the session
// from the request context.
package auth
