package sse

import (
	"net/http"
	"strconv"
	"strings"
)

// WithRequest supplies the request that opened the stream. The stream uses its
// Accept-Encoding header to negotiate gzip compression.
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

// acceptsGzip reports whether the Accept-Encoding header allows gzip.
func acceptsGzip(header string) bool {
	gzipQuality, wildcardQuality := -1.0, -1.0
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
		switch name {
		case "gzip", "x-gzip":
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}
	if gzipQuality >= 0 {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
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
