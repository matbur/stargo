package sse

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
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
	qualities, wildcard, hasWildcard := parseAcceptEncoding(header)
	best := ""
	bestQuality := 0.0
	for _, name := range supportedEncodings {
		quality, ok := qualities[name]
		if !ok && name == "gzip" {
			quality, ok = qualities["x-gzip"]
		}
		if !ok && hasWildcard {
			quality, ok = wildcard, true
		}
		if !ok || quality <= 0 || quality <= bestQuality {
			continue
		}
		best = name
		bestQuality = quality
	}
	return best
}

// parseAcceptEncoding returns explicit qualities and the wildcard quality.
func parseAcceptEncoding(header string) (map[string]float64, float64, bool) {
	qualities := make(map[string]float64)
	wildcard := 0.0
	hasWildcard := false
	for _, entry := range strings.Split(header, ",") {
		fields := strings.Split(entry, ";")
		name := strings.ToLower(strings.TrimSpace(fields[0]))
		if name == "" {
			continue
		}
		quality := 1.0
		valid := true
		for _, param := range fields[1:] {
			key, value, found := strings.Cut(param, "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				valid = false
				break
			}
			quality = parsed
		}
		if !valid {
			continue
		}
		if name == "*" {
			wildcard = quality
			hasWildcard = true
			continue
		}
		qualities[name] = quality
	}
	return qualities, wildcard, hasWildcard
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
