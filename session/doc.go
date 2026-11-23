// Package session keeps server-side state across requests.
//
// A Manager stores an opaque session id in a cookie, loads the matching record
// from a Store, and attaches a *Session to the request context. Ctx handlers
// read it with FromRequest:
//
//	s := session.FromRequest(c.R)
//	s.Set("user", "alice")
//
// The cookie is written when the session is first modified. Change the session
// before the response starts; a later write cannot change headers.
//
// Sessions expire after an idle timeout that slides on use and after an
// absolute lifetime anchored at creation. The default cookie is
// `__Host-session` with Secure, HttpOnly, and SameSite=Lax.
package session
