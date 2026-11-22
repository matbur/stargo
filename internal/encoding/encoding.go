// Package encoding negotiates content encodings from Accept-Encoding headers.
package encoding

import (
	"strconv"
	"strings"
)

// Select returns the most preferred accepted encoding from supported, or an
// empty string for identity.
//
// supported is ordered by server preference and is used as the tie-breaker.
// Qualities come from the request header, an explicit q=0 refuses an encoding,
// and the wildcard applies when an encoding is absent from the header.
// x-gzip is treated as an alias for gzip.
func Select(header string, supported ...string) string {
	qualities, wildcard, hasWildcard := parse(header)
	best := ""
	bestQuality := 0.0
	for _, name := range supported {
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

func parse(header string) (map[string]float64, float64, bool) {
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
