package route

import (
	"net/url"
	"strconv"
	"strings"
)

// Route is an immutable HTTP route handle.
//
// The zero value is invalid: declare routes with Get, Post, Put, Delete, Any,
// or New. Because all fields are unexported and hold only strings, two handles
// with the same method and path compare equal.
type Route struct {
	method  string
	path    string
	pattern string
}

// Get declares a route matching GET requests on path.
func Get(path string) Route { return New("GET", path) }

// Post declares a route matching POST requests on path.
func Post(path string) Route { return New("POST", path) }

// Put declares a route matching PUT requests on path.
func Put(path string) Route { return New("PUT", path) }

// Delete declares a route matching DELETE requests on path.
func Delete(path string) Route { return New("DELETE", path) }

// Any declares a route matching every method on path.
func Any(path string) Route { return New("", path) }

// New declares a route for an explicit method.
//
// The method is uppercased; an empty method matches every method. Construction
// panics on invalid input, because a route is a programmer-authored constant
// and a bad one must not start a server.
func New(method, path string) Route {
	method = strings.ToUpper(strings.TrimSpace(method))
	if strings.ContainsAny(method, " \t\r\n") {
		panic("route: method must be a single HTTP token")
	}
	if path == "" {
		panic("route: path is required")
	}
	if !strings.HasPrefix(path, "/") {
		panic("route: path must start with '/'")
	}
	if strings.HasPrefix(path, "//") {
		panic("route: host-qualified paths are not supported")
	}
	if strings.ContainsAny(path, " \t\r\n") {
		panic("route: path must not contain a method or whitespace")
	}
	if strings.ContainsAny(path, "?#") {
		panic("route: path must not contain a query or fragment")
	}

	path = normalizePath(path)

	// Validate placeholders once, at construction. URL building re-parses the
	// path from its canonical form.
	parseSegments(path)

	pattern := path
	if method != "" {
		pattern = method + " " + path
	}
	return Route{method: method, path: path, pattern: pattern}
}

// Method returns the HTTP method, or an empty string when the route matches
// every method.
func (r Route) Method() string { return r.method }

// Path returns the path template.
func (r Route) Path() string { return r.path }

// Pattern returns the combined pattern used for registration and span names.
func (r Route) Pattern() string { return r.pattern }

// HasParams reports whether the path template contains placeholders.
func (r Route) HasParams() bool {
	for _, segment := range parseSegments(r.path) {
		if segment.name != "" {
			return true
		}
	}
	return false
}

// Href builds a URL by substituting values for the path placeholders in order.
//
// Each value is percent-encoded so it cannot change the path structure. Href
// panics when the number of values does not match the number of placeholders,
// or when a static route receives values; both are programmer errors.
func (r Route) Href(values ...string) string {
	return r.URL(values...).String()
}

// URL starts a URL builder with path values substituted.
//
// Query parameters and a fragment are added with the builder's methods.
func (r Route) URL(values ...string) *URL {
	segments := parseSegments(r.path)
	params := 0
	for _, segment := range segments {
		if segment.name != "" {
			params++
		}
	}
	if len(values) != params {
		panic("route: expected " + strconv.Itoa(params) + " path values, got " + strconv.Itoa(len(values)))
	}

	var b strings.Builder
	b.Grow(len(r.path) + 16)
	if len(segments) == 0 {
		b.WriteString(r.path)
	} else {
		next := 0
		for _, segment := range segments {
			b.WriteByte('/')
			if segment.name == "" {
				b.WriteString(segment.literal)
				continue
			}
			b.WriteString(escapeValue(values[next], segment))
			next++
		}
	}
	return &URL{path: b.String(), query: url.Values{}}
}

// URL is an immutable URL builder for one route handle.
type URL struct {
	path     string
	query    url.Values
	fragment string
}

// Query adds query parameters. Keys are encoded in sorted order.
func (u *URL) Query(values url.Values) *URL {
	copied := *u
	copied.query = make(url.Values, len(u.query)+len(values))
	for key, value := range u.query {
		copied.query[key] = append([]string(nil), value...)
	}
	for key, value := range values {
		copied.query[key] = append([]string(nil), value...)
	}
	return &copied
}

// Fragment sets the fragment, escaped for a URL.
func (u *URL) Fragment(fragment string) *URL {
	copied := *u
	copied.fragment = fragment
	return &copied
}

// String returns the encoded URL.
func (u *URL) String() string {
	var b strings.Builder
	b.WriteString(u.path)
	if encoded := u.query.Encode(); encoded != "" {
		b.WriteByte('?')
		b.WriteString(encoded)
	}
	if u.fragment != "" {
		b.WriteByte('#')
		b.WriteString((&url.URL{Fragment: u.fragment}).EscapedFragment())
	}
	return b.String()
}

type segment struct {
	literal  string
	name     string
	catchall bool
}

// parseSegments returns one entry per path segment, in order. Static segments
// carry a literal; placeholder segments carry the placeholder name. An empty
// result means the route is exactly "/".
func parseSegments(path string) []segment {
	if path == "/" {
		return nil
	}
	parts := strings.Split(path[1:], "/")
	segments := make([]segment, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for i, part := range parts {
		if !strings.ContainsAny(part, "{}") {
			segments = append(segments, segment{literal: part})
			continue
		}
		if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' {
			panic("route: malformed placeholder in segment " + part)
		}
		name := part[1 : len(part)-1]
		if strings.ContainsAny(name, "{}") {
			panic("route: malformed placeholder in segment " + part)
		}
		catchall := strings.HasSuffix(name, "...")
		if catchall {
			name = strings.TrimSuffix(name, "...")
			if i != len(parts)-1 {
				panic("route: catch-all placeholder must be the last segment")
			}
		}
		if !validName(name) {
			panic("route: invalid placeholder name " + name)
		}
		if _, ok := seen[name]; ok {
			panic("route: duplicate placeholder name " + name)
		}
		seen[name] = struct{}{}
		segments = append(segments, segment{name: name, catchall: catchall})
	}
	return segments
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func escapeValue(value string, s segment) string {
	if value == "" {
		panic("route: path value for " + s.name + " must not be empty")
	}
	if s.catchall {
		parts := strings.Split(value, "/")
		for i, part := range parts {
			if part == "" || part == "." || part == ".." {
				panic("route: invalid catch-all path value for " + s.name)
			}
			parts[i] = url.PathEscape(part)
		}
		return strings.Join(parts, "/")
	}
	if value == "." || value == ".." {
		panic("route: path value for " + s.name + " is a dot segment")
	}
	return url.PathEscape(value)
}

func normalizePath(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}
