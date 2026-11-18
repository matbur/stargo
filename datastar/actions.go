package datastar

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ContentType is a Datastar fetch request content type.
type ContentType string

// Supported fetch content types.
const (
	ContentTypeJSON ContentType = "json"
	ContentTypeForm ContentType = "form"
)

// RetryMode controls when Datastar retries a fetch action.
type RetryMode string

// Supported retry modes.
const (
	RetryAuto   RetryMode = "auto"
	RetryError  RetryMode = "error"
	RetryAlways RetryMode = "always"
	RetryNever  RetryMode = "never"
)

// CancellationMode controls how a new request treats an in-flight one.
type CancellationMode string

// Supported request cancellation modes.
const (
	CancellationAuto     CancellationMode = "auto"
	CancellationCleanup  CancellationMode = "cleanup"
	CancellationDisabled CancellationMode = "disabled"
)

// ActionOption configures one fetch action.
type ActionOption func(*actionConfig)

type actionConfig struct {
	contentType       ContentType
	hasContentType    bool
	include           string
	hasInclude        bool
	exclude           string
	hasExclude        bool
	selector          string
	hasSelector       bool
	headers           map[string]string
	hasHeaders        bool
	openWhenHidden    bool
	hasOpenWhenHidden bool
	payload           map[string]string
	hasPayload        bool
	retry             RetryMode
	hasRetry          bool
	retryInterval     time.Duration
	hasRetryInterval  bool
	retryScaler       float64
	hasRetryScaler    bool
	retryMaxWait      time.Duration
	hasRetryMaxWait   bool
	retryMaxCount     int
	hasRetryMaxCount  bool
	cancellation      CancellationMode
	hasCancellation   bool
}

// Get builds a Datastar @get action.
func Get(path string, opts ...ActionOption) string {
	return fetchAction("get", path, opts)
}

// Post builds a Datastar @post action.
func Post(path string, opts ...ActionOption) string {
	return fetchAction("post", path, opts)
}

// Put builds a Datastar @put action.
func Put(path string, opts ...ActionOption) string {
	return fetchAction("put", path, opts)
}

// Patch builds a Datastar @patch action.
func Patch(path string, opts ...ActionOption) string {
	return fetchAction("patch", path, opts)
}

// Delete builds a Datastar @delete action.
func Delete(path string, opts ...ActionOption) string {
	return fetchAction("delete", path, opts)
}

// WithContentType sets the request content type.
func WithContentType(contentType ContentType) ActionOption {
	return func(c *actionConfig) {
		c.contentType = contentType
		c.hasContentType = true
	}
}

// WithForm sends a form-encoded request from the closest form.
func WithForm() ActionOption {
	return WithContentType(ContentTypeForm)
}

// WithSelector chooses the form a form request submits. It requires WithForm.
func WithSelector(selector string) ActionOption {
	return func(c *actionConfig) {
		c.selector = selector
		c.hasSelector = true
	}
}

// WithHeaders adds request headers. Values are literal strings.
func WithHeaders(headers map[string]string) ActionOption {
	return func(c *actionConfig) {
		c.headers = headers
		c.hasHeaders = true
	}
}

// WithOpenWhenHidden keeps the request open while the page is hidden.
func WithOpenWhenHidden(open bool) ActionOption {
	return func(c *actionConfig) {
		c.openWhenHidden = open
		c.hasOpenWhenHidden = true
	}
}

// WithPayload merges signal expressions into the request payload. Values are
// trusted JavaScript expressions, not strings.
func WithPayload(payload map[string]string) ActionOption {
	return func(c *actionConfig) {
		c.payload = payload
		c.hasPayload = true
	}
}

// WithSignalsInclude sends only signal paths matching the regular expression.
func WithSignalsInclude(pattern string) ActionOption {
	return func(c *actionConfig) {
		c.include = pattern
		c.hasInclude = true
	}
}

// WithSignalsExclude omits signal paths matching the regular expression.
func WithSignalsExclude(pattern string) ActionOption {
	return func(c *actionConfig) {
		c.exclude = pattern
		c.hasExclude = true
	}
}

// WithRetry sets the retry mode.
func WithRetry(mode RetryMode) ActionOption {
	return func(c *actionConfig) {
		c.retry = mode
		c.hasRetry = true
	}
}

// WithRetryInterval sets the initial retry wait.
func WithRetryInterval(d time.Duration) ActionOption {
	return func(c *actionConfig) {
		c.retryInterval = d
		c.hasRetryInterval = true
	}
}

// WithRetryScaler sets the retry wait multiplier.
func WithRetryScaler(scaler float64) ActionOption {
	return func(c *actionConfig) {
		c.retryScaler = scaler
		c.hasRetryScaler = true
	}
}

// WithRetryMaxWait sets the longest wait between retries.
func WithRetryMaxWait(d time.Duration) ActionOption {
	return func(c *actionConfig) {
		c.retryMaxWait = d
		c.hasRetryMaxWait = true
	}
}

// WithRetryMaxCount sets how many retries are attempted.
func WithRetryMaxCount(count int) ActionOption {
	return func(c *actionConfig) {
		c.retryMaxCount = count
		c.hasRetryMaxCount = true
	}
}

// WithRequestCancellation sets how a new request treats an in-flight one.
func WithRequestCancellation(mode CancellationMode) ActionOption {
	return func(c *actionConfig) {
		c.cancellation = mode
		c.hasCancellation = true
	}
}

func fetchAction(verb, path string, opts []ActionOption) string {
	var cfg actionConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.validate()

	var b strings.Builder
	b.WriteByte('@')
	b.WriteString(verb)
	b.WriteByte('(')
	b.WriteString(quoteJSString(path))
	if parts := cfg.optionParts(); len(parts) > 0 {
		b.WriteString(", {")
		b.WriteString(strings.Join(parts, ", "))
		b.WriteByte('}')
	}
	b.WriteByte(')')
	return b.String()
}

func (c actionConfig) validate() {
	if c.hasContentType {
		switch c.contentType {
		case ContentTypeJSON, ContentTypeForm:
		default:
			panic("datastar: unknown content type " + strconv.Quote(string(c.contentType)))
		}
	}
	if c.hasSelector {
		if c.contentType != ContentTypeForm || !c.hasContentType {
			panic("datastar: selector requires the form content type")
		}
		if c.selector == "" {
			panic("datastar: selector is required")
		}
	}
	if c.hasHeaders {
		for name := range c.headers {
			validateHeaderName(name)
		}
	}
	if c.hasPayload {
		for key := range c.payload {
			validatePayloadKey(key)
		}
	}
	if c.hasInclude {
		validatePattern("include", c.include)
	}
	if c.hasExclude {
		validatePattern("exclude", c.exclude)
	}
	if c.hasRetry {
		switch c.retry {
		case RetryAuto, RetryError, RetryAlways, RetryNever:
		default:
			panic("datastar: unknown retry mode " + strconv.Quote(string(c.retry)))
		}
	}
	if c.hasRetryInterval && c.retryInterval < time.Millisecond {
		panic("datastar: retry interval must be at least 1ms")
	}
	if c.hasRetryMaxWait && c.retryMaxWait < time.Millisecond {
		panic("datastar: retry max wait must be at least 1ms")
	}
	if c.hasRetryScaler && (math.IsNaN(c.retryScaler) || math.IsInf(c.retryScaler, 0) || c.retryScaler <= 0) {
		panic("datastar: retry scaler must be a finite positive number")
	}
	if c.hasRetryMaxCount && c.retryMaxCount < 0 {
		panic("datastar: retry max count must not be negative")
	}
	if c.hasCancellation {
		switch c.cancellation {
		case CancellationAuto, CancellationCleanup, CancellationDisabled:
		default:
			panic("datastar: unknown cancellation mode " + strconv.Quote(string(c.cancellation)))
		}
	}
}

func (c actionConfig) optionParts() []string {
	parts := make([]string, 0, 12)
	if c.hasContentType {
		parts = append(parts, "contentType: "+quoteJSString(string(c.contentType)))
	}
	if c.hasInclude || c.hasExclude {
		filters := make([]string, 0, 2)
		if c.hasInclude {
			filters = append(filters, "include: /"+c.include+"/")
		}
		if c.hasExclude {
			filters = append(filters, "exclude: /"+c.exclude+"/")
		}
		parts = append(parts, "filterSignals: {"+strings.Join(filters, ", ")+"}")
	}
	if c.hasSelector {
		parts = append(parts, "selector: "+quoteJSString(c.selector))
	}
	if c.hasHeaders {
		pairs := make([]string, 0, len(c.headers))
		for _, name := range sortedKeys(c.headers) {
			pairs = append(pairs, quoteJSString(name)+": "+quoteJSString(c.headers[name]))
		}
		parts = append(parts, "headers: {"+strings.Join(pairs, ", ")+"}")
	}
	if c.hasOpenWhenHidden {
		parts = append(parts, "openWhenHidden: "+strconv.FormatBool(c.openWhenHidden))
	}
	if c.hasPayload {
		pairs := make([]string, 0, len(c.payload))
		for _, key := range sortedKeys(c.payload) {
			pairs = append(pairs, key+": "+c.payload[key])
		}
		parts = append(parts, "payload: {"+strings.Join(pairs, ", ")+"}")
	}
	if c.hasRetry {
		parts = append(parts, "retry: "+quoteJSString(string(c.retry)))
	}
	if c.hasRetryInterval {
		parts = append(parts, "retryInterval: "+strconv.FormatInt(c.retryInterval.Milliseconds(), 10))
	}
	if c.hasRetryScaler {
		parts = append(parts, "retryScaler: "+strconv.FormatFloat(c.retryScaler, 'g', -1, 64))
	}
	if c.hasRetryMaxWait {
		parts = append(parts, "retryMaxWait: "+strconv.FormatInt(c.retryMaxWait.Milliseconds(), 10))
	}
	if c.hasRetryMaxCount {
		parts = append(parts, "retryMaxCount: "+strconv.Itoa(c.retryMaxCount))
	}
	if c.hasCancellation {
		parts = append(parts, "requestCancellation: "+quoteJSString(string(c.cancellation)))
	}
	return parts
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validateHeaderName(name string) {
	if name == "" {
		panic("datastar: header name is required")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		}
		panic("datastar: invalid header name " + strconv.Quote(name))
	}
}

func validatePayloadKey(key string) {
	if key == "" || strings.ContainsAny(key, " \t\r\n\"'{}") {
		panic("datastar: invalid payload key " + strconv.Quote(key))
	}
}

func validatePattern(name, pattern string) {
	if pattern == "" {
		panic("datastar: signal filter " + name + " pattern is required")
	}
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case 0:
			panic("datastar: signal filter " + name + " pattern must not contain NUL")
		case '\n', '\r':
			panic("datastar: signal filter " + name + " pattern must be a single line")
		case '/':
			if i == 0 || pattern[i-1] != '\\' {
				panic("datastar: signal filter " + name + " pattern must escape /")
			}
		}
	}
	if strings.ContainsAny(pattern, "\u2028\u2029") {
		panic("datastar: signal filter " + name + " pattern must be a single line")
	}
}
