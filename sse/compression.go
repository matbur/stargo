package sse

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/matbur/stargo/internal/encoding"
)

// compressor is the stream encoding seam shared by gzip and brotli.
type compressor interface {
	io.Writer
	Flush() error
	Close() error
}

// supportedEncodings is ordered by server preference.
var supportedEncodings = []string{"br", "gzip"}

// WithRequest supplies the request that opened the stream. The stream uses its
// Accept-Encoding header to negotiate brotli or gzip compression.
//
// Ctx.Stream passes the request automatically. Direct sse.New callers opt in
// by passing it; without a request the stream stays uncompressed.
func WithRequest(r *http.Request) StreamOption {
	return func(s *Stream) {
		if r != nil {
			s.request = r
		}
	}
}

// selectEncoding returns the preferred accepted encoding, or "" for identity.
func selectEncoding(header string) string {
	return encoding.Select(header, supportedEncodings...)
}

func newCompressor(encoding string, w io.Writer) compressor {
	switch encoding {
	case "br":
		return brotli.NewWriter(w)
	case "gzip":
		return gzip.NewWriter(w)
	default:
		return nil
	}
}

// addVary adds Accept-Encoding to the response Vary header once.
func addVary(header http.Header) {
	for _, value := range header.Values("Vary") {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "Accept-Encoding") {
				return
			}
		}
	}
	header.Add("Vary", "Accept-Encoding")
}

// hasDirective reports whether a comma-separated header contains the directive.
func hasDirective(value, directive string) bool {
	for _, token := range strings.Split(value, ",") {
		name, _, _ := strings.Cut(token, "=")
		if strings.EqualFold(strings.TrimSpace(name), directive) {
			return true
		}
	}
	return false
}

// ensureNoCache preserves existing cache directives and adds no-cache.
func ensureNoCache(header http.Header) {
	if hasDirective(header.Get("Cache-Control"), "no-cache") {
		return
	}
	if current := strings.TrimSpace(header.Get("Cache-Control")); current != "" {
		header.Set("Cache-Control", current+", no-cache")
		return
	}
	header.Set("Cache-Control", "no-cache")
}
