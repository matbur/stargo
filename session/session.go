package session

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/matbur/stargo/cookies"
)

// ErrDestroyed is returned by Rotate after the session was destroyed.
var ErrDestroyed = errors.New("session: destroyed")

type contextKey struct{}

// Session is the state of one request's session.
//
// A session is created by Manager middleware. Data access is safe for
// concurrent use, but the response cookie is written by the first modification,
// so change the session before the response starts.
type Session struct {
	manager *Manager
	writer  http.ResponseWriter

	mu          sync.Mutex
	id          string
	data        map[string]any
	created     time.Time
	storedID    string
	destroyIDs  []string
	new         bool
	used        bool
	destroyed   bool
	rotated     bool
	hadCookie   bool
	cookieWrote bool
	cookieErr   error
}

// FromContext returns the session attached by Manager middleware, or nil.
func FromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	value, _ := ctx.Value(contextKey{}).(*Session)
	return value
}

// FromRequest returns the session attached by Manager middleware, or nil.
func FromRequest(request *http.Request) *Session {
	if request == nil {
		return nil
	}
	return FromContext(request.Context())
}

// Value reads a typed value from the session.
//
// The second result is false when the key is absent or holds another type.
func Value[T any](s *Session, key string) (T, bool) {
	var zero T
	if s == nil {
		return zero, false
	}
	value, ok := s.Get(key)
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

// ID returns the current session id.
func (s *Session) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// Get returns the value stored under key.
func (s *Session) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.data[key]
	return value, ok
}

// Data returns a copy of the session values.
func (s *Session) Data() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneValues(s.data)
}

// Set stores a value and marks the session for saving.
//
// On a new session the cookie is written immediately, so call Set before the
// response starts.
func (s *Session) Set(key string, value any) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed {
		return
	}
	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = value
	s.used = true
	if s.new && !s.cookieWrote {
		s.writeCookieLocked()
	}
}

// Delete removes a key and marks the session for saving when it existed.
func (s *Session) Delete(key string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed {
		return
	}
	if _, ok := s.data[key]; !ok {
		return
	}
	delete(s.data, key)
	s.used = true
}

// Clear removes every value and marks the session for saving.
func (s *Session) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed || len(s.data) == 0 {
		return
	}
	s.data = make(map[string]any)
	s.used = true
}

// Rotate replaces the session id while keeping the data and deleting the
// previous record.
//
// Rotate after a privilege change, such as a successful login. It returns an
// error when the session was destroyed or a new id cannot be generated.
func (s *Session) Rotate() error {
	if s == nil {
		return ErrDestroyed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed {
		return ErrDestroyed
	}
	id, err := s.manager.newID()
	if err != nil {
		return err
	}
	if s.storedID != "" {
		s.destroyIDs = append(s.destroyIDs, s.storedID)
		s.storedID = ""
	}
	s.id = id
	s.rotated = true
	s.used = true
	s.writeCookieLocked()
	return s.cookieErr
}

// Destroy deletes the session and clears the cookie.
//
// Call Destroy before the response starts, for example in a logout handler.
func (s *Session) Destroy() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed {
		return
	}
	if s.storedID != "" {
		s.destroyIDs = append(s.destroyIDs, s.storedID)
		s.storedID = ""
	}
	s.destroyed = true
	s.used = true
	if s.hadCookie || s.cookieWrote {
		s.clearCookieLocked()
	}
}

func (s *Session) writeCookieLocked() {
	if s.writer == nil {
		return
	}
	s.cookieWrote = true
	if err := cookies.Set(s.writer, s.manager.cookie(s.id)); err != nil && s.cookieErr == nil {
		s.cookieErr = err
	}
}

func (s *Session) clearCookieLocked() {
	if s.writer == nil {
		return
	}
	s.cookieWrote = false
	s.hadCookie = false
	if err := cookies.Delete(s.writer, s.manager.cookie(s.id)); err != nil && s.cookieErr == nil {
		s.cookieErr = err
	}
}
