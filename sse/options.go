package sse

import "fmt"

// Mode is a Datastar element patch mode.
type Mode string

// Supported element patch modes.
const (
	ModeOuter   Mode = "outer"
	ModeInner   Mode = "inner"
	ModeReplace Mode = "replace"
	ModePrepend Mode = "prepend"
	ModeAppend  Mode = "append"
	ModeBefore  Mode = "before"
	ModeAfter   Mode = "after"
	ModeRemove  Mode = "remove"
)

// Namespace is a Datastar patch namespace.
type Namespace string

// Supported patch namespaces.
const (
	NamespaceSVG    Namespace = "svg"
	NamespaceMathML Namespace = "mathml"
)

// Options carries the fields shared by Datastar SSE events.
type Options struct {
	Mode                   Mode
	Selector               string
	Namespace              Namespace
	UseViewTransition      bool
	ViewTransitionSelector string
	EventID                string
	Retry                  int
	OnlyIfMissing          bool
	AutoRemove             bool
}

// Option configures one event.
type Option func(*Options)

// WithMode sets the element patch mode.
func WithMode(mode Mode) Option {
	return func(o *Options) { o.Mode = mode }
}

// WithSelector sets the CSS selector the patch targets.
func WithSelector(selector string) Option {
	return func(o *Options) { o.Selector = selector }
}

// WithNamespace sets the patch namespace.
func WithNamespace(namespace Namespace) Option {
	return func(o *Options) { o.Namespace = namespace }
}

// WithViewTransition enables the browser view transition for this patch.
func WithViewTransition() Option {
	return func(o *Options) { o.UseViewTransition = true }
}

// WithViewTransitionSelector targets the view transition at a selector.
func WithViewTransitionSelector(selector string) Option {
	return func(o *Options) { o.ViewTransitionSelector = selector }
}

// WithEventID sets the SSE event id.
func WithEventID(id string) Option {
	return func(o *Options) { o.EventID = id }
}

// WithRetry sets the reconnection retry interval in milliseconds. The
// protocol default of 1000 is omitted from the event.
func WithRetry(milliseconds int) Option {
	return func(o *Options) { o.Retry = milliseconds }
}

// WithOnlyIfMissing patches signals only when they are absent in the browser.
func WithOnlyIfMissing() Option {
	return func(o *Options) { o.OnlyIfMissing = true }
}

// WithAutoRemove controls whether an executed script removes itself. Scripts
// remove themselves by default.
func WithAutoRemove(autoRemove bool) Option {
	return func(o *Options) {
		o.AutoRemove = autoRemove
	}
}

func resolveOptions(opts []Option) Options {
	options := Options{AutoRemove: true}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

func (o Options) validateMode() error {
	if o.Mode == "" {
		return nil
	}
	switch o.Mode {
	case ModeOuter, ModeInner, ModeReplace, ModePrepend, ModeAppend, ModeBefore, ModeAfter, ModeRemove:
		return nil
	}
	return fmt.Errorf("sse: unknown patch mode %q", o.Mode)
}

func (o Options) validateNamespace() error {
	switch o.Namespace {
	case "", NamespaceSVG, NamespaceMathML:
		return nil
	}
	return fmt.Errorf("sse: unknown patch namespace %q", o.Namespace)
}

func (o Options) validateRetry() error {
	if o.Retry < 0 {
		return fmt.Errorf("sse: retry must be a positive number of milliseconds")
	}
	return nil
}
