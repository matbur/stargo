// Package forms reads HTML form submissions.
//
// It accepts the two encodings a browser sends:
// application/x-www-form-urlencoded and multipart/form-data. GET and HEAD read
// the query string; every other method reads the request body. Parsing is
// bounded by default, and the caller closes the form to release temporary
// upload files.
//
// Values and filenames are untrusted. Validate them, and never use a filename
// as a path without further sanitizing it.
//
// Import it as forms:
//
//	import "github.com/matbur/stargo/forms"
//
//	form, err := forms.Read(c.R)
//	if err != nil {
//		return err
//	}
//	defer form.Close()
//
//	name := form.Get("name")
package forms

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

const (
	// DefaultMaxBytes is the default limit of a form body: 10 MiB.
	DefaultMaxBytes int64 = 10 << 20
	// DefaultMaxFields is the default limit of text fields in one form: 1000.
	DefaultMaxFields = 1000
	// DefaultMaxFieldBytes is the default limit of one text field: 1 MiB.
	DefaultMaxFieldBytes int64 = 1 << 20
	// DefaultMaxFiles is the default limit of uploaded files in one form: 10.
	DefaultMaxFiles = 10
	// DefaultMemoryBytes is the default limit of one upload kept in memory
	// before it moves to a temporary file: 1 MiB.
	DefaultMemoryBytes int64 = 1 << 20
)

var (
	// ErrMalformed is returned when a form body cannot be decoded.
	ErrMalformed = errors.New("forms: malformed form body")
	// ErrTooLarge is returned when a form exceeds a configured limit.
	ErrTooLarge = errors.New("forms: form body too large")
	// ErrUnsupportedMediaType is returned when the request does not carry a
	// supported form body.
	ErrUnsupportedMediaType = errors.New("forms: expected a form body")
	// ErrNoFile is returned when an upload without a file is opened.
	ErrNoFile = errors.New("forms: no file to open")
)

const (
	urlEncodedMediaType = "application/x-www-form-urlencoded"
	multipartMediaType  = "multipart/form-data"
)

// Option configures Read.
type Option func(*config)

type config struct {
	maxBytes      int64
	maxFields     int
	maxFieldBytes int64
	maxFiles      int
	maxFileBytes  int64
	memoryBytes   int64
}

func defaultConfig() config {
	return config{
		maxBytes:      DefaultMaxBytes,
		maxFields:     DefaultMaxFields,
		maxFieldBytes: DefaultMaxFieldBytes,
		maxFiles:      DefaultMaxFiles,
		maxFileBytes:  0,
		memoryBytes:   DefaultMemoryBytes,
	}
}

// WithMaxBytes limits the whole form body. The default is DefaultMaxBytes.
func WithMaxBytes(bytes int64) Option {
	checkLimit(bytes)
	return func(c *config) { c.maxBytes = bytes }
}

// WithMaxFields limits the number of text fields. The default is
// DefaultMaxFields.
func WithMaxFields(fields int) Option {
	checkLimit(int64(fields))
	return func(c *config) { c.maxFields = fields }
}

// WithMaxFieldBytes limits one text field. The default is
// DefaultMaxFieldBytes. For a URL-encoded form the limit applies to the decoded
// value; for a multipart form it applies to the bytes of the part.
func WithMaxFieldBytes(bytes int64) Option {
	checkLimit(bytes)
	return func(c *config) { c.maxFieldBytes = bytes }
}

// WithMaxFiles limits the number of uploaded files. The default is
// DefaultMaxFiles.
func WithMaxFiles(files int) Option {
	checkLimit(int64(files))
	return func(c *config) { c.maxFiles = files }
}

// WithMaxFileBytes limits one uploaded file. The default is zero, which means
// no separate limit below the body limit.
func WithMaxFileBytes(bytes int64) Option {
	checkLimit(bytes)
	return func(c *config) { c.maxFileBytes = bytes }
}

// WithMemoryBytes limits how much of one upload stays in memory before it moves
// to a temporary file. The default is DefaultMemoryBytes.
func WithMemoryBytes(bytes int64) Option {
	checkLimit(bytes)
	return func(c *config) { c.memoryBytes = bytes }
}

func checkLimit(value int64) {
	if value < 0 {
		panic("forms: negative limit")
	}
}

// Read parses the form submitted with a request.
//
// GET and HEAD read the query string; every other method reads the body, which
// must be application/x-www-form-urlencoded or multipart/form-data. The body is
// consumed. On success the returned form owns any temporary upload files, so
// the caller must close it:
//
//	form, err := forms.Read(r)
//	if err != nil {
//		return err
//	}
//	defer form.Close()
//
// Errors wrap ErrMalformed, ErrTooLarge, or ErrUnsupportedMediaType, so a
// handler can map them to 400, 413, and 415.
func Read(request *http.Request, opts ...Option) (*Form, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: nil request", ErrMalformed)
	}
	config := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	switch request.Method {
	case http.MethodGet, http.MethodHead:
		return readQuery(request.URL, config)
	default:
		return readBody(request, config)
	}
}

func readQuery(target *url.URL, config config) (*Form, error) {
	values := url.Values{}
	if target != nil {
		values = target.Query()
	}
	if err := checkValues(values, config); err != nil {
		return nil, err
	}
	return &Form{values: values}, nil
}

func readBody(request *http.Request, config config) (*Form, error) {
	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedMediaType, err)
	}
	if charset := params["charset"]; charset != "" &&
		!strings.EqualFold(charset, "utf-8") && !strings.EqualFold(charset, "utf8") {
		return nil, fmt.Errorf("%w: charset %q", ErrUnsupportedMediaType, charset)
	}

	switch mediaType {
	case urlEncodedMediaType:
		return readURLEncoded(request, config)
	case multipartMediaType:
		return readMultipart(request, config)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMediaType, mediaType)
	}
}

func readURLEncoded(request *http.Request, config config) (*Form, error) {
	raw, err := readBounded(request.Body, config.maxBytes)
	if err != nil {
		return nil, err
	}
	values, err := url.ParseQuery(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if err := checkValues(values, config); err != nil {
		return nil, err
	}
	return &Form{values: values}, nil
}

func readBounded(body io.Reader, maxBytes int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	// Read one byte past the limit so an oversized body is detected without
	// buffering it whole.
	raw, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("%w: body exceeds %d bytes", ErrTooLarge, maxBytes)
	}
	return raw, nil
}

func readMultipart(request *http.Request, config config) (*Form, error) {
	if request.Body == nil {
		request.Body = http.NoBody
	}
	request.Body = http.MaxBytesReader(nil, request.Body, config.maxBytes)
	if err := request.ParseMultipartForm(config.memoryBytes); err != nil {
		if parsed := request.MultipartForm; parsed != nil {
			_ = parsed.RemoveAll()
		}
		var maxError *http.MaxBytesError
		if errors.As(err, &maxError) {
			return nil, fmt.Errorf("%w: body exceeds %d bytes", ErrTooLarge, config.maxBytes)
		}
		if errors.Is(err, http.ErrNotMultipart) || errors.Is(err, http.ErrMissingBoundary) {
			return nil, fmt.Errorf("%w: %v", ErrUnsupportedMediaType, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	parsed := request.MultipartForm
	form := &Form{
		values:    parsed.Value,
		files:     uploadFiles(parsed.File),
		multipart: parsed,
	}
	if err := form.checkLimits(config); err != nil {
		_ = form.Close()
		return nil, err
	}
	return form, nil
}

func checkValues(values map[string][]string, config config) error {
	count := 0
	for name, list := range values {
		count += len(list)
		if count > config.maxFields {
			return fmt.Errorf("%w: more than %d fields", ErrTooLarge, config.maxFields)
		}
		for _, value := range list {
			if int64(len(value)) > config.maxFieldBytes {
				return fmt.Errorf("%w: field %q exceeds %d bytes", ErrTooLarge, name, config.maxFieldBytes)
			}
		}
	}
	return nil
}
