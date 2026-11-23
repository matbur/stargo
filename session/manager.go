package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/matbur/stargo/cookies"
)

const (
	defaultCookieName = "__Host-session"
	devCookieName     = "session"
	defaultIdle       = 30 * time.Minute
	defaultAbsolute   = 24 * time.Hour
)

// Option configures a Manager.
type Option func(*Manager)

// WithCookieName sets the session cookie name. The default is `__Host-session`.
func WithCookieName(name string) Option {
	return func(m *Manager) { m.name = name }
}

// WithIdleTimeout sets how long a session may stay unused. The default is 30
// minutes. The value must be positive.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(m *Manager) { m.idle = timeout }
}

// WithAbsoluteTimeout sets the maximum lifetime from creation. The default is
// 24 hours. Zero disables the limit.
func WithAbsoluteTimeout(timeout time.Duration) Option {
	return func(m *Manager) { m.absolute = timeout }
}

// WithPath sets the cookie path. The default is `/`.
func WithPath(path string) Option {
	return func(m *Manager) { m.path = path }
}

// WithDomain sets the cookie domain. The default is no domain.
func WithDomain(domain string) Option {
	return func(m *Manager) { m.domain = domain }
}

// WithSecure sets the Secure flag. It is true by default.
func WithSecure(secure bool) Option {
	return func(m *Manager) { m.secure = secure }
}

// WithHTTPOnly sets the HttpOnly flag. It is true by default.
func WithHTTPOnly(httpOnly bool) Option {
	return func(m *Manager) { m.httpOnly = httpOnly }
}

// WithSameSite sets the SameSite value. The default is SameSite=Lax.
func WithSameSite(sameSite http.SameSite) Option {
	return func(m *Manager) { m.sameSite = sameSite }
}

// WithClock replaces the time source. Tests use it to advance expiration.
func WithClock(now func() time.Time) Option {
	return func(m *Manager) {
		if now != nil {
			m.now = now
		}
	}
}

// WithErrorHandler sets the callback for commit and cookie errors.
//
// Commit errors happen after the handler, when the response may already be
// written, so they cannot change the status. The default logs through
// log/slog. Passing nil disables the callback.
func WithErrorHandler(handler func(error)) Option {
	return func(m *Manager) {
		if handler == nil {
			m.onError = func(error) {}
			return
		}
		m.onError = handler
	}
}

// Insecure turns off Secure and, while the name is still the default,
// renames the cookie to `session`.
//
// Use it for local HTTP development only. Browsers reject a `__Host-` cookie
// without Secure, so the default name cannot stay.
func Insecure() Option {
	return func(m *Manager) {
		m.secure = false
		if m.name == defaultCookieName {
			m.name = devCookieName
		}
	}
}

// Manager loads, saves, and expires sessions.
//
// A Manager is immutable after New and safe for concurrent use.
type Manager struct {
	store    Store
	name     string
	idle     time.Duration
	absolute time.Duration
	path     string
	domain   string
	secure   bool
	httpOnly bool
	sameSite http.SameSite
	now      func() time.Time
	onError  func(error)
}

// New returns a Manager backed by store.
//
// Defaults are the `__Host-session` cookie with Secure, HttpOnly, SameSite=Lax,
// Path=/, and no Domain, a 30-minute idle timeout, and a 24-hour absolute
// lifetime.
func New(store Store, opts ...Option) (*Manager, error) {
	if store == nil {
		return nil, errors.New("session: store is nil")
	}
	manager := &Manager{
		store:    store,
		name:     defaultCookieName,
		idle:     defaultIdle,
		absolute: defaultAbsolute,
		path:     "/",
		secure:   true,
		httpOnly: true,
		sameSite: http.SameSiteLaxMode,
		now:      time.Now,
		onError: func(err error) {
			slog.Default().Error("stargo: session", "error", err)
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}
	if manager.idle <= 0 {
		return nil, fmt.Errorf("session: idle timeout must be positive, got %s", manager.idle)
	}
	if manager.absolute < 0 {
		return nil, fmt.Errorf("session: absolute timeout must not be negative, got %s", manager.absolute)
	}
	if manager.name == "" {
		return nil, errors.New("session: cookie name is empty")
	}
	if err := validateCookie(manager.cookie("session-id")); err != nil {
		return nil, err
	}
	return manager, nil
}

// Middleware loads the session, attaches it to the request context, and
// commits it after the handler.
//
// The returned function has the signature of stargo.Middleware, so an
// application registers it with App.Use.
func (m *Manager) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, err := m.load(w, r)
			if err != nil {
				m.report(err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer func() {
				if err := m.commit(r.Context(), s); err != nil {
					m.report(err)
				}
			}()
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, s)))
		})
	}
}

// CookieName returns the configured cookie name.
func (m *Manager) CookieName() string {
	return m.name
}

func (m *Manager) load(w http.ResponseWriter, r *http.Request) (*Session, error) {
	now := m.now()
	id := cookies.Read(r)[m.name]
	s := &Session{
		manager:   m,
		writer:    w,
		data:      make(map[string]any),
		created:   now,
		hadCookie: id != "",
	}
	if id != "" {
		record, ok, err := m.store.Load(r.Context(), id)
		if err != nil {
			return nil, err
		}
		if ok {
			created := record.Created
			if created.IsZero() {
				created = now
			}
			absoluteExpired := m.absolute > 0 && !now.Before(created.Add(m.absolute))
			if record.Expires.After(now) && !absoluteExpired {
				s.id = id
				s.storedID = id
				s.data = cloneValues(record.Values)
				s.created = created
				s.used = true
				return s, nil
			}
			_ = m.store.Delete(r.Context(), id)
		}
	}

	newID, err := m.newID()
	if err != nil {
		return nil, err
	}
	s.id = newID
	s.new = true
	return s, nil
}

func (m *Manager) commit(ctx context.Context, s *Session) error {
	s.mu.Lock()
	used := s.used
	destroyed := s.destroyed
	id := s.id
	created := s.created
	values := cloneValues(s.data)
	destroyIDs := append([]string(nil), s.destroyIDs...)
	cookieErr := s.cookieErr
	s.mu.Unlock()

	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	keep(cookieErr)
	for _, oldID := range destroyIDs {
		keep(m.store.Delete(ctx, oldID))
	}
	if destroyed || !used {
		return firstErr
	}

	now := m.now()
	expires := now.Add(m.idle)
	if m.absolute > 0 {
		absolute := created.Add(m.absolute)
		if absolute.Before(expires) {
			expires = absolute
		}
	}
	if !expires.After(now) {
		keep(m.store.Delete(ctx, id))
		return firstErr
	}
	keep(m.store.Save(ctx, id, Record{Values: values, Created: created, Expires: expires}))
	return firstErr
}

func (m *Manager) cookie(id string) *http.Cookie {
	return &http.Cookie{
		Name:     m.name,
		Value:    id,
		Path:     m.path,
		Domain:   m.domain,
		Secure:   m.secure,
		HttpOnly: m.httpOnly,
		SameSite: m.sameSite,
	}
}

func (m *Manager) newID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("session: generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (m *Manager) report(err error) {
	if err != nil && m.onError != nil {
		m.onError(err)
	}
}

func validateCookie(cookie *http.Cookie) error {
	if err := cookies.Set(nullWriter{}, cookie); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	return nil
}

type nullWriter struct{}

func (nullWriter) Header() http.Header            { return make(http.Header) }
func (nullWriter) Write(body []byte) (int, error) { return len(body), nil }
func (nullWriter) WriteHeader(int)                {}
