package datastar

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// IntersectOption configures one viewport intersection attribute.
type IntersectOption func(*intersectConfig)

type intersectConfig struct {
	threshold      string
	hasThreshold   bool
	once           bool
	exit           bool
	delay          time.Duration
	hasDelay       bool
	debounce       time.Duration
	hasDebounce    bool
	throttle       time.Duration
	hasThrottle    bool
	viewTransition bool
}

func (c *intersectConfig) setThreshold(modifier string) {
	if c.hasThreshold {
		panic("datastar: intersect threshold is already set")
	}
	c.threshold = modifier
	c.hasThreshold = true
}

// WithIntersectThreshold watches for the given visible fraction, between 0 and
// 1, and renders it as a percentage.
func WithIntersectThreshold(fraction float64) IntersectOption {
	return func(c *intersectConfig) {
		if math.IsNaN(fraction) || math.IsInf(fraction, 0) || fraction < 0 || fraction > 1 {
			panic("datastar: intersect threshold must be between 0 and 1")
		}
		c.setThreshold("threshold." + strconv.Itoa(int(math.Round(fraction*100))))
	}
}

// WithIntersectHalf watches for half of the element to be visible.
func WithIntersectHalf() IntersectOption {
	return func(c *intersectConfig) { c.setThreshold("half") }
}

// WithIntersectFull watches for the whole element to be visible.
func WithIntersectFull() IntersectOption {
	return func(c *intersectConfig) { c.setThreshold("full") }
}

// WithIntersectThresholdKeyword renders threshold.<keyword> for a custom
// Datastar threshold value. The keyword must be lowercase kebab-case.
func WithIntersectThresholdKeyword(keyword string) IntersectOption {
	return func(c *intersectConfig) {
		validateAttributeKey(keyword)
		c.setThreshold("threshold." + keyword)
	}
}

// WithIntersectOnce reacts at most once.
func WithIntersectOnce() IntersectOption {
	return func(c *intersectConfig) { c.once = true }
}

// WithIntersectExit reacts when the element leaves the viewport.
func WithIntersectExit() IntersectOption {
	return func(c *intersectConfig) { c.exit = true }
}

// WithIntersectDelay delays the reaction by d.
func WithIntersectDelay(d time.Duration) IntersectOption {
	return func(c *intersectConfig) {
		c.delay = d
		c.hasDelay = true
	}
}

// WithIntersectDebounce debounces the reaction by d.
func WithIntersectDebounce(d time.Duration) IntersectOption {
	return func(c *intersectConfig) {
		c.debounce = d
		c.hasDebounce = true
	}
}

// WithIntersectThrottle throttles the reaction by d.
func WithIntersectThrottle(d time.Duration) IntersectOption {
	return func(c *intersectConfig) {
		c.throttle = d
		c.hasThrottle = true
	}
}

// WithIntersectViewTransition runs the expression in a view transition.
func WithIntersectViewTransition() IntersectOption {
	return func(c *intersectConfig) { c.viewTransition = true }
}

// OnIntersect renders a data-on-intersect attribute with the standard data-
// prefix.
func OnIntersect(expression string, opts ...IntersectOption) Attribute {
	return Default().OnIntersect(expression, opts...)
}

// OnIntersect renders a viewport intersection listener.
//
// The expression is trusted JavaScript and is HTML-escaped as an attribute
// value. Invalid options panic.
func (a Attributes) OnIntersect(expression string, opts ...IntersectOption) Attribute {
	var cfg intersectConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	modifiers := make([]string, 0, 7)
	if cfg.hasThreshold {
		modifiers = append(modifiers, cfg.threshold)
	}
	if cfg.once {
		modifiers = append(modifiers, "once")
	}
	if cfg.exit {
		modifiers = append(modifiers, "exit")
	}
	if cfg.hasDelay {
		modifiers = append(modifiers, "delay."+milliseconds("intersect delay", cfg.delay))
	}
	if cfg.hasDebounce {
		modifiers = append(modifiers, "debounce."+milliseconds("intersect debounce", cfg.debounce))
	}
	if cfg.hasThrottle {
		modifiers = append(modifiers, "throttle."+milliseconds("intersect throttle", cfg.throttle))
	}
	if cfg.viewTransition {
		modifiers = append(modifiers, "viewtransition")
	}
	return a.named(modifierName("on-intersect", modifiers), expression)
}

func modifierName(name string, modifiers []string) string {
	if len(modifiers) == 0 {
		return name
	}
	return name + "__" + strings.Join(modifiers, "__")
}

// intervalDuration renders whole seconds as Ns and everything else as Nms.
func intervalDuration(option string, d time.Duration) string {
	if d < time.Millisecond {
		panic("datastar: " + option + " must be at least 1ms")
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
}

// IntervalOption configures one interval attribute.
type IntervalOption func(*intervalConfig)

type intervalConfig struct {
	duration       time.Duration
	hasDuration    bool
	leading        bool
	viewTransition bool
}

// WithIntervalDuration sets the interval duration. The default is one second.
func WithIntervalDuration(d time.Duration) IntervalOption {
	return func(c *intervalConfig) {
		c.duration = d
		c.hasDuration = true
	}
}

// WithIntervalLeading runs the expression immediately, before the first
// interval elapses.
func WithIntervalLeading() IntervalOption {
	return func(c *intervalConfig) { c.leading = true }
}

// WithIntervalViewTransition runs the expression in a view transition.
func WithIntervalViewTransition() IntervalOption {
	return func(c *intervalConfig) { c.viewTransition = true }
}

// OnInterval renders a data-on-interval attribute with the standard data-
// prefix.
func OnInterval(expression string, opts ...IntervalOption) Attribute {
	return Default().OnInterval(expression, opts...)
}

// OnInterval renders an interval listener.
//
// One second is Datastar's default and is omitted unless leading is requested,
// in which case the duration is explicit. The expression is trusted JavaScript
// and is HTML-escaped as an attribute value. Invalid options panic.
func (a Attributes) OnInterval(expression string, opts ...IntervalOption) Attribute {
	var cfg intervalConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	modifiers := make([]string, 0, 2)
	if cfg.leading || (cfg.hasDuration && cfg.duration != time.Second) {
		duration := time.Second
		if cfg.hasDuration {
			duration = cfg.duration
		}
		modifier := "duration." + intervalDuration("interval duration", duration)
		if cfg.leading {
			modifier += ".leading"
		}
		modifiers = append(modifiers, modifier)
	}
	if cfg.viewTransition {
		modifiers = append(modifiers, "viewtransition")
	}
	return a.named(modifierName("on-interval", modifiers), expression)
}

// SignalPatchOption configures one signal patch attribute.
type SignalPatchOption func(*signalPatchConfig)

type signalPatchConfig struct {
	delay       time.Duration
	hasDelay    bool
	debounce    time.Duration
	hasDebounce bool
	throttle    time.Duration
	hasThrottle bool
	filters     signalFilters
}

// WithSignalPatchDelay delays the reaction by d.
func WithSignalPatchDelay(d time.Duration) SignalPatchOption {
	return func(c *signalPatchConfig) {
		c.delay = d
		c.hasDelay = true
	}
}

// WithSignalPatchDebounce debounces the reaction by d.
func WithSignalPatchDebounce(d time.Duration) SignalPatchOption {
	return func(c *signalPatchConfig) {
		c.debounce = d
		c.hasDebounce = true
	}
}

// WithSignalPatchThrottle throttles the reaction by d.
func WithSignalPatchThrottle(d time.Duration) SignalPatchOption {
	return func(c *signalPatchConfig) {
		c.throttle = d
		c.hasThrottle = true
	}
}

// WithSignalPatchInclude limits the reaction to signals matching pattern.
func WithSignalPatchInclude(pattern string) SignalPatchOption {
	return func(c *signalPatchConfig) { c.filters.setInclude(pattern) }
}

// WithSignalPatchExclude skips signals matching pattern.
func WithSignalPatchExclude(pattern string) SignalPatchOption {
	return func(c *signalPatchConfig) { c.filters.setExclude(pattern) }
}

// OnSignalPatch renders a data-on-signal-patch attribute with the standard
// data- prefix.
func OnSignalPatch(expression string, opts ...SignalPatchOption) Group {
	return Default().OnSignalPatch(expression, opts...)
}

// OnSignalPatch renders a signal patch listener and, when filters are set, the
// companion data-on-signal-patch-filter attribute.
//
// The expression is trusted JavaScript and is HTML-escaped as an attribute
// value. Patterns are trusted regular expressions; an invalid pattern or a
// duration shorter than one millisecond panics.
func (a Attributes) OnSignalPatch(expression string, opts ...SignalPatchOption) Group {
	var cfg signalPatchConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	modifiers := make([]string, 0, 3)
	if cfg.hasDelay {
		modifiers = append(modifiers, "delay."+milliseconds("signal patch delay", cfg.delay))
	}
	if cfg.hasDebounce {
		modifiers = append(modifiers, "debounce."+milliseconds("signal patch debounce", cfg.debounce))
	}
	if cfg.hasThrottle {
		modifiers = append(modifiers, "throttle."+milliseconds("signal patch throttle", cfg.throttle))
	}
	group := Group{a.named(modifierName("on-signal-patch", modifiers), expression)}
	if filters, ok := cfg.filters.object(); ok {
		group = append(group, a.named("on-signal-patch-filter", filters))
	}
	return group
}
