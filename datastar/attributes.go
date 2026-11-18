package datastar

import (
	"io"
	"strconv"
	"strings"
	"time"

	g "maragu.dev/gomponents"
)

// Attributes renders Datastar event attributes with one attribute prefix.
type Attributes struct {
	prefix string
}

// Default returns attributes for the standard data- prefix.
func Default() Attributes {
	return Attributes{prefix: "data-"}
}

// New returns attributes for a Datastar bundle built with an aliased prefix.
//
// The prefix must match the bundle the page loads, and the same prefix must be
// used for initial pages, SSE patches, and raw HTML. A missing trailing hyphen
// is added. New panics when the prefix is empty or invalid.
func New(prefix string) Attributes {
	return Attributes{prefix: normalizePrefix(prefix)}
}

// On renders a data-on attribute with the standard data- prefix.
func On(event, expression string, opts ...OnOption) Attr {
	return Default().On(event, expression, opts...)
}

// On renders a data-on attribute.
//
// Event names are lowercase kebab-case. The expression is trusted JavaScript
// and is HTML-escaped as an attribute value. Invalid or contradictory options
// panic.
func (a Attributes) On(event, expression string, opts ...OnOption) Attr {
	if a.prefix == "" {
		panic("datastar: use Default or New to create Attributes")
	}
	validateEvent(event)
	var cfg onConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return Attr{name: a.prefix + "on:" + event + modifierSuffix(cfg), value: expression}
}

// Attr is one rendered HTML attribute.
type Attr struct {
	name  string
	value string
}

// Render writes the attribute, including its leading space.
func (a Attr) Render(w io.Writer) error {
	var b strings.Builder
	b.Grow(len(a.name) + len(a.value) + 4)
	b.WriteByte(' ')
	b.WriteString(a.name)
	b.WriteString(`="`)
	b.WriteString(escapeAttribute(a.value))
	b.WriteByte('"')
	_, err := io.WriteString(w, b.String())
	return err
}

// Type reports an attribute node to renderers that distinguish attributes
// from elements, such as gomponents.
func (Attr) Type() g.NodeType {
	return g.AttributeType
}

// Case is a Datastar event key case modifier.
type Case string

// Supported event key cases.
const (
	CaseKebab  Case = "kebab"
	CaseCamel  Case = "camel"
	CaseSnake  Case = "snake"
	CasePascal Case = "pascal"
)

// OnOption configures one event attribute.
type OnOption func(*onConfig)

type onConfig struct {
	once           bool
	passive        bool
	capture        bool
	stop           bool
	prevent        bool
	target         string
	delay          time.Duration
	hasDelay       bool
	debounce       time.Duration
	hasDebounce    bool
	throttle       time.Duration
	hasThrottle    bool
	viewTransition bool
	caseMode       Case
}

// WithOnce makes the listener run at most once.
func WithOnce() OnOption {
	return func(c *onConfig) { c.once = true }
}

// WithPassive marks the listener as passive.
func WithPassive() OnOption {
	return func(c *onConfig) { c.passive = true }
}

// WithCapture evaluates the listener during the capture phase.
func WithCapture() OnOption {
	return func(c *onConfig) { c.capture = true }
}

// WithStop stops event propagation.
func WithStop() OnOption {
	return func(c *onConfig) { c.stop = true }
}

// WithPrevent calls preventDefault before the expression runs.
func WithPrevent() OnOption {
	return func(c *onConfig) { c.prevent = true }
}

// WithWindow attaches the listener to the window.
func WithWindow() OnOption {
	return func(c *onConfig) { c.target = "window" }
}

// WithDocument attaches the listener to the document.
func WithDocument() OnOption {
	return func(c *onConfig) { c.target = "document" }
}

// WithOutside attaches the listener outside the element.
func WithOutside() OnOption {
	return func(c *onConfig) { c.target = "outside" }
}

// WithDelay delays the listener by d.
func WithDelay(d time.Duration) OnOption {
	return func(c *onConfig) {
		c.delay = d
		c.hasDelay = true
	}
}

// WithDebounce debounces the listener by d.
func WithDebounce(d time.Duration) OnOption {
	return func(c *onConfig) {
		c.debounce = d
		c.hasDebounce = true
	}
}

// WithThrottle throttles the listener by d.
func WithThrottle(d time.Duration) OnOption {
	return func(c *onConfig) {
		c.throttle = d
		c.hasThrottle = true
	}
}

// WithViewTransition runs the expression in a view transition.
func WithViewTransition() OnOption {
	return func(c *onConfig) { c.viewTransition = true }
}

// WithCase converts the event key to the given case on the client.
func WithCase(caseMode Case) OnOption {
	return func(c *onConfig) { c.caseMode = caseMode }
}

func modifierSuffix(cfg onConfig) string {
	if cfg.passive && cfg.prevent {
		panic("datastar: passive and prevent contradict each other")
	}
	modifiers := make([]string, 0, 11)
	if cfg.once {
		modifiers = append(modifiers, "once")
	}
	if cfg.passive {
		modifiers = append(modifiers, "passive")
	}
	if cfg.capture {
		modifiers = append(modifiers, "capture")
	}
	if cfg.target != "" {
		modifiers = append(modifiers, cfg.target)
	}
	if cfg.prevent {
		modifiers = append(modifiers, "prevent")
	}
	if cfg.stop {
		modifiers = append(modifiers, "stop")
	}
	if cfg.hasDelay {
		modifiers = append(modifiers, "delay."+milliseconds("delay", cfg.delay))
	}
	if cfg.hasDebounce {
		modifiers = append(modifiers, "debounce."+milliseconds("debounce", cfg.debounce))
	}
	if cfg.hasThrottle {
		modifiers = append(modifiers, "throttle."+milliseconds("throttle", cfg.throttle))
	}
	if cfg.viewTransition {
		modifiers = append(modifiers, "viewtransition")
	}
	if cfg.caseMode != "" {
		validateCase(cfg.caseMode)
		if cfg.caseMode != CaseKebab {
			modifiers = append(modifiers, "case."+string(cfg.caseMode))
		}
	}
	if len(modifiers) == 0 {
		return ""
	}
	return "__" + strings.Join(modifiers, "__")
}

func milliseconds(option string, d time.Duration) string {
	if d < time.Millisecond {
		panic("datastar: " + option + " must be at least 1ms")
	}
	return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
}

func validateCase(caseMode Case) {
	switch caseMode {
	case CaseKebab, CaseCamel, CaseSnake, CasePascal:
		return
	}
	panic("datastar: unknown case " + strconv.Quote(string(caseMode)))
}

func validateEvent(event string) {
	if validEvent(event) {
		return
	}
	panic("datastar: event name " + strconv.Quote(event) + " must be lowercase kebab-case")
}

func validEvent(event string) bool {
	if event == "" || event[0] < 'a' || event[0] > 'z' || event[len(event)-1] == '-' {
		return false
	}
	for i := 0; i < len(event); i++ {
		c := event[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}

func normalizePrefix(prefix string) string {
	if prefix == "" {
		panic("datastar: prefix is required")
	}
	if !strings.HasSuffix(prefix, "-") {
		prefix += "-"
	}
	if !strings.HasPrefix(prefix, "data-") {
		panic("datastar: prefix " + strconv.Quote(prefix) + " must start with data-")
	}
	alias := strings.TrimSuffix(strings.TrimPrefix(prefix, "data-"), "-")
	if prefix != "data-" && alias == "" {
		panic("datastar: prefix " + strconv.Quote(prefix) + " must include an alias")
	}
	if alias != "" && !validEvent(alias) {
		panic("datastar: prefix " + strconv.Quote(prefix) + " must be lowercase kebab-case")
	}
	return prefix
}
