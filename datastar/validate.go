package datastar

import (
	"strconv"
	"strings"
)

// validateAttributeKey accepts only lowercase kebab-case keys, so a key
// cannot inject additional attribute syntax.
func validateAttributeKey(key string) {
	if key == "" || key[0] == '-' || key[len(key)-1] == '-' {
		panic("datastar: attribute key " + strconv.Quote(key) + " must be lowercase kebab-case")
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		panic("datastar: attribute key " + strconv.Quote(key) + " must be lowercase kebab-case")
	}
}

// validateSignalName accepts a non-empty dot-separated signal path. Names are
// case-sensitive because signals are JavaScript identifiers.
func validateSignalName(name string) {
	invalid := func() {
		panic("datastar: signal name " + strconv.Quote(name) + " must be a dot-separated path")
	}
	if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		invalid()
	}
	for _, segment := range strings.Split(name, ".") {
		if segment == "" {
			invalid()
		}
		for i := 0; i < len(segment); i++ {
			c := segment[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
				continue
			}
			invalid()
		}
	}
}
