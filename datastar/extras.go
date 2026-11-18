package datastar

import (
	"strconv"
	"strings"
)

// signalFilters holds the include and exclude patterns of one signal filter
// object.
type signalFilters struct {
	include    string
	hasInclude bool
	exclude    string
	hasExclude bool
}

// object renders the filter object, or reports false when neither pattern is
// set.
func (f signalFilters) object() (string, bool) {
	if !f.hasInclude && !f.hasExclude {
		return "", false
	}
	parts := make([]string, 0, 2)
	if f.hasInclude {
		parts = append(parts, quoteJSString("include")+":"+quoteJSString(f.include))
	}
	if f.hasExclude {
		parts = append(parts, quoteJSString("exclude")+":"+quoteJSString(f.exclude))
	}
	return "{" + strings.Join(parts, ",") + "}", true
}

func (f *signalFilters) setInclude(pattern string) {
	validatePattern("include", pattern)
	f.include = pattern
	f.hasInclude = true
}

func (f *signalFilters) setExclude(pattern string) {
	validatePattern("exclude", pattern)
	f.exclude = pattern
	f.hasExclude = true
}

// IgnoreOption configures one ignore attribute.
type IgnoreOption func(*ignoreConfig)

type ignoreConfig struct {
	self bool
}

// WithIgnoreSelf skips Datastar processing for this element only, not its
// subtree.
func WithIgnoreSelf() IgnoreOption {
	return func(c *ignoreConfig) { c.self = true }
}

// Ignore renders a data-ignore attribute with the standard data- prefix.
func Ignore(opts ...IgnoreOption) Attribute {
	return Default().Ignore(opts...)
}

// Ignore renders an attribute that skips Datastar processing.
func (a Attributes) Ignore(opts ...IgnoreOption) Attribute {
	var cfg ignoreConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	suffix := "ignore"
	if cfg.self {
		suffix += "__self"
	}
	return a.bareNamed(suffix)
}

// IgnoreMorph renders a data-ignore-morph attribute with the standard data-
// prefix.
func IgnoreMorph() Attribute {
	return Default().IgnoreMorph()
}

// IgnoreMorph renders an attribute that prevents backend patches from morphing
// the subtree.
func (a Attributes) IgnoreMorph() Attribute {
	return a.bareNamed("ignore-morph")
}

// Indicator renders a data-indicator attribute with the standard data- prefix.
func Indicator(signalName string) Attribute {
	return Default().Indicator(signalName)
}

// Indicator renders an attribute that tracks in-flight requests in a signal.
func (a Attributes) Indicator(signalName string) Attribute {
	validateSignalName(signalName)
	return a.named("indicator", signalName)
}

// Ref renders a data-ref attribute with the standard data- prefix.
func Ref(signalName string) Attribute {
	return Default().Ref(signalName)
}

// Ref renders an attribute that stores the current element in a signal.
func (a Attributes) Ref(signalName string) Attribute {
	validateSignalName(signalName)
	return a.named("ref", signalName)
}

// PersistOption configures one persist attribute.
type PersistOption func(*persistConfig)

type persistConfig struct {
	filters signalFilters
	key     string
	hasKey  bool
	session bool
}

// WithPersistInclude persists only signals matching pattern.
func WithPersistInclude(pattern string) PersistOption {
	return func(c *persistConfig) { c.filters.setInclude(pattern) }
}

// WithPersistExclude skips signals matching pattern.
func WithPersistExclude(pattern string) PersistOption {
	return func(c *persistConfig) { c.filters.setExclude(pattern) }
}

// WithPersistKey stores the signals under key in the browser storage.
func WithPersistKey(key string) PersistOption {
	return func(c *persistConfig) {
		c.key = key
		c.hasKey = true
	}
}

// WithPersistSession uses session storage instead of local storage.
func WithPersistSession() PersistOption {
	return func(c *persistConfig) { c.session = true }
}

// Persist renders a data-persist attribute with the standard data- prefix.
func Persist(opts ...PersistOption) Attribute {
	return Default().Persist(opts...)
}

// Persist renders an attribute that persists signals in browser storage.
//
// Without filters the attribute has no value. Patterns are trusted regular
// expressions; an invalid pattern or storage key panics.
func (a Attributes) Persist(opts ...PersistOption) Attribute {
	var cfg persistConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	suffix := "persist"
	if cfg.hasKey {
		validateAttributeKey(cfg.key)
		suffix += ":" + cfg.key
	}
	if cfg.session {
		suffix += "__session"
	}
	if filters, ok := cfg.filters.object(); ok {
		return a.named(suffix, filters)
	}
	return a.bareNamed(suffix)
}

// PreserveAttr renders a data-preserve-attr attribute with the standard data-
// prefix.
func PreserveAttr(names ...string) Attribute {
	return Default().PreserveAttr(names...)
}

// PreserveAttr renders the attribute names that DOM morphing must preserve.
//
// At least one name is required. Names are space-separated in the value, so a
// name containing whitespace, control characters, quotes, '=', '<', '>', '/',
// or '&' panics. Uppercase and namespaced names such as viewBox are allowed.
func (a Attributes) PreserveAttr(names ...string) Attribute {
	if len(names) == 0 {
		panic("datastar: preserve attrs needs at least one attribute name")
	}
	for _, name := range names {
		validatePreservedAttribute(name)
	}
	return a.named("preserve-attr", strings.Join(names, " "))
}

func validatePreservedAttribute(name string) {
	if name == "" {
		panic("datastar: preserved attribute name is required")
	}
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c < 0x21, c == 0x7f:
			panic("datastar: preserved attribute name " + strconv.Quote(name) + " contains an invalid character")
		case c == '"', c == '\'', c == '=', c == '<', c == '>', c == '/', c == '&':
			panic("datastar: preserved attribute name " + strconv.Quote(name) + " contains an invalid character")
		}
	}
}

// JSONSignalsOption configures one JSON signals attribute.
type JSONSignalsOption func(*jsonSignalsConfig)

type jsonSignalsConfig struct {
	filters signalFilters
	terse   bool
}

// WithJSONSignalsInclude renders only signals matching pattern.
func WithJSONSignalsInclude(pattern string) JSONSignalsOption {
	return func(c *jsonSignalsConfig) { c.filters.setInclude(pattern) }
}

// WithJSONSignalsExclude skips signals matching pattern.
func WithJSONSignalsExclude(pattern string) JSONSignalsOption {
	return func(c *jsonSignalsConfig) { c.filters.setExclude(pattern) }
}

// WithJSONSignalsTerse renders the minified terse form.
func WithJSONSignalsTerse() JSONSignalsOption {
	return func(c *jsonSignalsConfig) { c.terse = true }
}

// JSONSignals renders a data-json-signals attribute with the standard data-
// prefix.
func JSONSignals(opts ...JSONSignalsOption) Attribute {
	return Default().JSONSignals(opts...)
}

// JSONSignals renders signals as JSON text.
//
// Without filters the attribute has no value. Patterns are trusted regular
// expressions; an invalid pattern panics.
func (a Attributes) JSONSignals(opts ...JSONSignalsOption) Attribute {
	var cfg jsonSignalsConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	suffix := "json-signals"
	if cfg.terse {
		suffix += "__terse"
	}
	if filters, ok := cfg.filters.object(); ok {
		return a.named(suffix, filters)
	}
	return a.bareNamed(suffix)
}
