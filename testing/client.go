package stargotest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync"
)

// TB is the subset of testing.TB the helpers need.
//
// *testing.T and *testing.B satisfy it. The interface is local because
// testing.TB has an unexported method, so tests outside package testing cannot
// implement it to record failures.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// Client is an HTTP test client bound to a loopback test server.
type Client struct {
	tb         TB
	server     *httptest.Server
	httpClient *http.Client
	base       *url.URL
	defaults   http.Header

	closeOnce sync.Once
}

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	cookies      bool
	redirects    bool
	maxRedirects int
	headers      http.Header
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		cookies:      true,
		redirects:    true,
		maxRedirects: 10,
		headers:      make(http.Header),
	}
}

// WithoutCookies disables the cookie jar. Response cookies are ignored and
// later requests carry no stored cookies.
func WithoutCookies() Option {
	return func(config *clientConfig) {
		config.cookies = false
	}
}

// WithoutRedirects stops following redirects. The 3xx response is returned
// unchanged instead of an error.
func WithoutRedirects() Option {
	return func(config *clientConfig) {
		config.redirects = false
	}
}

// WithMaxRedirects caps how many redirects a request follows. Exceeding the
// limit fails the test. A limit below one stops at the first redirect.
func WithMaxRedirects(limit int) Option {
	return func(config *clientConfig) {
		config.redirects = true
		config.maxRedirects = limit
	}
}

// WithDefaultHeader adds a header to every request. A header set on one
// request with WithHeader overrides the default.
func WithDefaultHeader(name, value string) Option {
	return func(config *clientConfig) {
		config.headers.Set(name, value)
	}
}

// NewClient starts a test server for handler and returns a client for it.
//
// The server and its idle connections are closed when the test finishes, or
// earlier through Close.
func NewClient(tb TB, handler http.Handler, opts ...Option) *Client {
	tb.Helper()

	config := defaultClientConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	server := httptest.NewServer(handler)
	httpClient := *server.Client()
	if config.cookies {
		jar, err := cookiejar.New(nil)
		if err != nil {
			server.Close()
			tb.Fatalf("stargotest: create cookie jar: %v", err)
			return nil
		}
		httpClient.Jar = jar
	}
	if !config.redirects {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		limit := config.maxRedirects
		httpClient.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= limit {
				return fmt.Errorf("stopped after %d redirects", limit)
			}
			return nil
		}
	}

	base, err := url.Parse(server.URL)
	if err != nil {
		server.Close()
		tb.Fatalf("stargotest: parse test server URL %q: %v", server.URL, err)
		return nil
	}
	base.Path = "/"

	client := &Client{
		tb:         tb,
		server:     server,
		httpClient: &httpClient,
		base:       base,
		defaults:   config.headers,
	}
	tb.Cleanup(client.Close)
	return client
}

// URL resolves target against the test server base URL. Absolute URLs are
// returned unchanged.
func (c *Client) URL(target string) string {
	c.tb.Helper()

	reference, err := url.Parse(target)
	if err != nil {
		c.tb.Fatalf("stargotest: parse %q: %v", target, err)
		return ""
	}
	if reference.IsAbs() {
		return reference.String()
	}
	return c.base.ResolveReference(reference).String()
}

// Close stops the test server and closes its idle connections. It is safe to
// call more than once.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.httpClient.CloseIdleConnections()
		c.server.Close()
	})
}

// Cookie returns a stored cookie value and whether the jar holds the cookie.
func (c *Client) Cookie(name string) (string, bool) {
	c.tb.Helper()

	if c.httpClient.Jar == nil {
		return "", false
	}
	for _, cookie := range c.httpClient.Jar.Cookies(c.base) {
		if cookie.Name == name {
			return cookie.Value, true
		}
	}
	return "", false
}

// RequestOption configures one request.
type RequestOption func(*requestSpec)

type requestSpec struct {
	header      http.Header
	body        []byte
	bodySet     bool
	contentType string
	err         error
}

// WithHeader sets a request header. It overrides a default header with the
// same name.
func WithHeader(name, value string) RequestOption {
	return func(spec *requestSpec) {
		spec.header.Set(name, value)
	}
}

// WithBody sets a request body and its media type.
func WithBody(contentType string, body []byte) RequestOption {
	return func(spec *requestSpec) {
		spec.body = body
		spec.bodySet = true
		spec.contentType = contentType
	}
}

// WithJSON encodes value as a JSON request body.
func WithJSON(value any) RequestOption {
	return func(spec *requestSpec) {
		body, err := json.Marshal(value)
		if err != nil {
			spec.err = fmt.Errorf("encode JSON: %w", err)
			return
		}
		spec.body = body
		spec.bodySet = true
		spec.contentType = "application/json"
	}
}

// WithForm encodes values as an application/x-www-form-urlencoded body.
func WithForm(values url.Values) RequestOption {
	return func(spec *requestSpec) {
		spec.body = []byte(values.Encode())
		spec.bodySet = true
		spec.contentType = "application/x-www-form-urlencoded"
	}
}

// WithMultipart encodes form as a multipart/form-data body with a generated
// boundary.
func WithMultipart(form Multipart) RequestOption {
	return func(spec *requestSpec) {
		contentType, body, err := encodeMultipart(form)
		if err != nil {
			spec.err = fmt.Errorf("encode multipart: %w", err)
			return
		}
		spec.body = body
		spec.bodySet = true
		spec.contentType = contentType
	}
}

// Request builds a request for target without sending it.
func (c *Client) Request(method, target string, opts ...RequestOption) *http.Request {
	c.tb.Helper()

	spec := requestSpec{header: make(http.Header)}
	for _, opt := range opts {
		if opt != nil {
			opt(&spec)
		}
	}
	if spec.err != nil {
		c.tb.Fatalf("stargotest: %s %s: %v", method, target, spec.err)
		return nil
	}

	var body io.Reader
	if spec.bodySet {
		body = bytes.NewReader(spec.body)
	}
	request, err := http.NewRequest(method, c.URL(target), body)
	if err != nil {
		c.tb.Fatalf("stargotest: build %s %s: %v", method, target, err)
		return nil
	}
	for name, values := range spec.header {
		request.Header[name] = append([]string(nil), values...)
	}
	if spec.contentType != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", spec.contentType)
	}
	for name, values := range c.defaults {
		if request.Header.Get(name) == "" {
			request.Header[name] = append([]string(nil), values...)
		}
	}
	return request
}

// Do sends request and returns its buffered response.
func (c *Client) Do(request *http.Request) *Response {
	c.tb.Helper()

	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		c.tb.Fatalf("stargotest: %s %s: %v", request.Method, request.URL, err)
		return nil
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		c.tb.Fatalf("stargotest: %s %s: read body: %v", request.Method, request.URL, err)
		return nil
	}
	return &Response{
		tb:         c.tb,
		StatusCode: httpResponse.StatusCode,
		Header:     httpResponse.Header.Clone(),
		Body:       body,
		Request:    request,
	}
}

// Get sends a GET request.
func (c *Client) Get(target string, opts ...RequestOption) *Response {
	c.tb.Helper()
	return c.Do(c.Request(http.MethodGet, target, opts...))
}

// PostForm sends a form request with an
// application/x-www-form-urlencoded body.
func (c *Client) PostForm(target string, values url.Values, opts ...RequestOption) *Response {
	c.tb.Helper()
	return c.Do(c.Request(http.MethodPost, target, append(append([]RequestOption(nil), opts...), WithForm(values))...))
}

// PostMultipart sends a multipart/form-data request.
func (c *Client) PostMultipart(target string, form Multipart, opts ...RequestOption) *Response {
	c.tb.Helper()
	return c.Do(c.Request(http.MethodPost, target, append(append([]RequestOption(nil), opts...), WithMultipart(form))...))
}
