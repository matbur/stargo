// Package relay implements a process-local publish/subscribe.
package relay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrClosed is returned by Next after the subscription is closed.
var ErrClosed = errors.New("relay: subscription closed")

// Relay is a process-local pub/sub hub. The zero value is not usable; call New.
type Relay[T any] struct {
	mu   sync.RWMutex
	subs map[*Subscription[T]]struct{}
}

// New returns an empty relay.
func New[T any]() *Relay[T] {
	return &Relay[T]{subs: make(map[*Subscription[T]]struct{})}
}

// Publish sends value to every subscription matching subject.
//
// It never waits for a subscriber: each subscription keeps one pending value
// per subject and a bounded number of subjects. Publishing to an invalid
// subject is a programmer error and panics.
func (r *Relay[T]) Publish(subject string, value T) {
	if !validSubject(subject) {
		panic("relay: invalid subject " + subject)
	}

	r.mu.RLock()
	subs := make([]*Subscription[T], 0, len(r.subs))
	for sub := range r.subs {
		subs = append(subs, sub)
	}
	r.mu.RUnlock()

	for _, sub := range subs {
		if match(sub.pattern, subject) {
			sub.deliver(subject, value)
		}
	}
}

// Subscribe registers a subscription for one pattern.
//
// A pattern is an exact dotted subject, a trailing prefix wildcard
// (`room.*`), or the global wildcard (`*`). The context is checked once at
// registration; pass the same context to Next to stop waiting on cancellation.
func (r *Relay[T]) Subscribe(ctx context.Context, pattern string, opts ...Option) (*Subscription[T], error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	if !validPattern(pattern) {
		return nil, fmt.Errorf("relay: invalid pattern %q", pattern)
	}

	config := subscriptionConfig{maxSize: 64}
	for _, opt := range opts {
		opt(&config)
	}

	sub := &Subscription[T]{
		relay:   r,
		pattern: pattern,
		maxSize: config.maxSize,
		pending: make(map[string]T),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	r.mu.Lock()
	r.subs[sub] = struct{}{}
	r.mu.Unlock()
	return sub, nil
}

func (r *Relay[T]) remove(sub *Subscription[T]) {
	r.mu.Lock()
	delete(r.subs, sub)
	r.mu.Unlock()
}

// Option configures a subscription.
type Option func(*subscriptionConfig)

type subscriptionConfig struct {
	maxSize int
}

// MaxSize bounds how many subjects may be pending at once. When the limit is
// reached, publishing a new subject drops the oldest pending one.
func MaxSize(n int) Option {
	return func(c *subscriptionConfig) {
		if n > 0 {
			c.maxSize = n
		}
	}
}

// Latest keeps only the most recently published subject pending. It is a
// convenient MaxSize(1).
func Latest() Option {
	return MaxSize(1)
}

// Subscription is one subscriber's pending values.
type Subscription[T any] struct {
	relay   *Relay[T]
	pattern string
	maxSize int

	mu      sync.Mutex
	pending map[string]T
	order   []string
	dropped uint64
	closed  bool
	wake    chan struct{}
	done    chan struct{}
}

// Dropped returns the number of pending values replaced or evicted by the
// bounds policy.
func (s *Subscription[T]) Dropped() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Next waits for the next pending subject and returns its newest value.
//
// It blocks until a value is pending, ctx is cancelled, or the subscription is
// closed. Pending values are consumed in publication order; a newer value for
// a subject moves that subject to the end.
func (s *Subscription[T]) Next(ctx context.Context) (subject string, value T, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		s.mu.Lock()
		if len(s.order) > 0 {
			subject = s.order[0]
			s.order = s.order[1:]
			value = s.pending[subject]
			delete(s.pending, subject)
			s.mu.Unlock()
			return subject, value, nil
		}
		if s.closed {
			s.mu.Unlock()
			return "", value, ErrClosed
		}
		s.mu.Unlock()

		select {
		case <-s.wake:
		case <-ctx.Done():
			return "", value, ctx.Err()
		case <-s.done:
			return "", value, ErrClosed
		}
	}
}

// Close unregisters the subscription and discards pending values. It is safe
// to call more than once.
func (s *Subscription[T]) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.pending = nil
	s.order = nil
	close(s.done)
	s.mu.Unlock()

	s.relay.remove(s)
}

func (s *Subscription[T]) deliver(subject string, value T) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	if _, exists := s.pending[subject]; exists {
		s.pending[subject] = value
		for i, name := range s.order {
			if name == subject {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
		s.order = append(s.order, subject)
	} else {
		if len(s.order) >= s.maxSize {
			oldest := s.order[0]
			s.order = s.order[1:]
			delete(s.pending, oldest)
			s.dropped++
		}
		s.pending[subject] = value
		s.order = append(s.order, subject)
	}
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// match reports whether subject matches pattern.
func match(pattern, subject string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		// Keep the trailing dot so "ab.*" does not match "abc.x".
		return strings.HasPrefix(subject, pattern[:len(pattern)-1])
	}
	return pattern == subject
}

func validSubject(subject string) bool {
	if subject == "" || strings.Contains(subject, "*") {
		return false
	}
	if strings.HasPrefix(subject, ".") || strings.HasSuffix(subject, ".") {
		return false
	}
	return !strings.Contains(subject, "..")
}

func validPattern(pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		return validSubject(pattern[:len(pattern)-2])
	}
	return validSubject(pattern)
}
