package datastar

import (
	"strings"
	"time"
)

// InitOption configures one initialization attribute.
type InitOption func(*initConfig)

type initConfig struct {
	delay          time.Duration
	hasDelay       bool
	viewTransition bool
}

// WithInitDelay delays the initialization expression by d.
func WithInitDelay(d time.Duration) InitOption {
	return func(c *initConfig) {
		c.delay = d
		c.hasDelay = true
	}
}

// WithInitViewTransition runs the initialization expression in a view
// transition.
func WithInitViewTransition() InitOption {
	return func(c *initConfig) { c.viewTransition = true }
}

// Init renders a data-init attribute with the standard data- prefix.
func Init(expression string, opts ...InitOption) Attribute {
	return Default().Init(expression, opts...)
}

// Init renders a data-init attribute.
func (a Attributes) Init(expression string, opts ...InitOption) Attribute {
	var cfg initConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	modifiers := make([]string, 0, 2)
	if cfg.hasDelay {
		modifiers = append(modifiers, "delay."+milliseconds("init delay", cfg.delay))
	}
	if cfg.viewTransition {
		modifiers = append(modifiers, "viewtransition")
	}
	suffix := "init"
	if len(modifiers) > 0 {
		suffix += "__" + strings.Join(modifiers, "__")
	}
	return a.named(suffix, expression)
}

// BindOption configures one two-way binding.
type BindOption func(*bindConfig)

type bindConfig struct {
	prop     string
	hasProp  bool
	event    string
	hasEvent bool
}

// WithBindProp binds a specific element property instead of its value.
func WithBindProp(prop string) BindOption {
	return func(c *bindConfig) {
		c.prop = prop
		c.hasProp = true
	}
}

// WithBindEvent binds on a specific event instead of the default one.
func WithBindEvent(event string) BindOption {
	return func(c *bindConfig) {
		c.event = event
		c.hasEvent = true
	}
}

// Bind renders a data-bind attribute with the standard data- prefix.
func Bind(signalName string, opts ...BindOption) Attribute {
	return Default().Bind(signalName, opts...)
}

// Bind renders a two-way binding attribute for signalName.
func (a Attributes) Bind(signalName string, opts ...BindOption) Attribute {
	validateSignalName(signalName)
	var cfg bindConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	modifiers := make([]string, 0, 2)
	if cfg.hasProp {
		validateAttributeKey(cfg.prop)
		modifiers = append(modifiers, "prop."+cfg.prop)
	}
	if cfg.hasEvent {
		validateAttributeKey(cfg.event)
		modifiers = append(modifiers, "event."+cfg.event)
	}
	suffix := "bind"
	if len(modifiers) > 0 {
		suffix += "__" + strings.Join(modifiers, "__")
	}
	return a.named(suffix, signalName)
}

// Attr renders a data-attr:<key> attribute with the standard data- prefix.
func Attr(key, expression string) Attribute {
	return Default().Attr(key, expression)
}

// Attr renders one HTML attribute from a reactive expression.
func (a Attributes) Attr(key, expression string) Attribute {
	validateAttributeKey(key)
	return a.named("attr:"+key, expression)
}

// Show renders a data-show attribute with the standard data- prefix.
func Show(expression string) Attribute {
	return Default().Show(expression)
}

// Show renders an attribute that shows or hides the element.
func (a Attributes) Show(expression string) Attribute {
	return a.named("show", expression)
}

// Text renders a data-text attribute with the standard data- prefix.
func Text(expression string) Attribute {
	return Default().Text(expression)
}

// Text renders an attribute that binds the element text content.
func (a Attributes) Text(expression string) Attribute {
	return a.named("text", expression)
}

// Effect renders a data-effect attribute with the standard data- prefix.
func Effect(expression string) Attribute {
	return Default().Effect(expression)
}

// Effect renders an attribute that runs an expression on initialization and
// updates.
func (a Attributes) Effect(expression string) Attribute {
	return a.named("effect", expression)
}

// Class renders a data-class:<name> attribute with the standard data- prefix.
func Class(name, expression string) Attribute {
	return Default().Class(name, expression)
}

// Class renders one reactive CSS class toggle.
func (a Attributes) Class(name, expression string) Attribute {
	validateAttributeKey(name)
	return a.named("class:"+name, expression)
}

// Style renders a data-style:<property> attribute with the standard data-
// prefix.
func Style(property, expression string) Attribute {
	return Default().Style(property, expression)
}

// Style renders one reactive inline style property.
func (a Attributes) Style(property, expression string) Attribute {
	validateAttributeKey(property)
	return a.named("style:"+property, expression)
}

// Computed renders a data-computed attribute with the standard data- prefix.
func Computed(name, expression string) Attribute {
	return Default().Computed(name, expression)
}

// Computed renders one computed signal as an arrow function.
//
// A dotted name expands into a nested object, so ui.total addresses
// $ui.total.
func (a Attributes) Computed(name, expression string) Attribute {
	return a.named("computed", computedExpression(name, expression))
}

func computedExpression(name, expression string) string {
	validateSignalName(name)
	parts := strings.Split(name, ".")
	inner := "() => (" + expression + ")"
	for i := len(parts) - 1; i >= 0; i-- {
		inner = "{" + quoteJSString(parts[i]) + ":" + inner + "}"
	}
	return inner
}

// named renders one attribute with this instance's prefix.
func (a Attributes) named(suffix, value string) Attribute {
	if a.prefix == "" {
		panic("datastar: use Default or New to create Attributes")
	}
	return Attribute{name: a.prefix + suffix, value: value}
}
