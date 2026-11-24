package auth

import (
	"net/http"
)

// Option configures RequireUser.
type Option func(*config)

type config struct {
	redirect     string
	unauthorized http.Handler
}

// RedirectTo sends unauthenticated requests to path with 303 See Other.
//
// A redirect wins over a custom unauthorized handler. An empty path disables
// the redirect.
func RedirectTo(path string) Option {
	return func(c *config) { c.redirect = path }
}

// Unauthorized replaces the default 401 response. A nil handler is ignored.
func Unauthorized(handler http.Handler) Option {
	return func(c *config) {
		if handler != nil {
			c.unauthorized = handler
		}
	}
}

// RequireUser returns middleware that calls the next handler only when the
// request has an authenticated user.
//
// Without a configured redirect or unauthorized handler it answers 401 with a
// short text body. Register the session middleware first so the session is in
// the request context.
func RequireUser(opts ...Option) func(http.Handler) http.Handler {
	config := config{
		unauthorized: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := UserFromRequest(r); ok {
				next.ServeHTTP(w, r)
				return
			}
			if config.redirect != "" {
				http.Redirect(w, r, config.redirect, http.StatusSeeOther)
				return
			}
			config.unauthorized.ServeHTTP(w, r)
		})
	}
}
