package route

import (
	"net/url"
	"reflect"
	"testing"
)

func TestConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		route   Route
		method  string
		path    string
		pattern string
		params  bool
	}{
		{"get", Get("/rooms/{id}"), "GET", "/rooms/{id}", "GET /rooms/{id}", true},
		{"post", Post("/rooms"), "POST", "/rooms", "POST /rooms", false},
		{"put", Put("/rooms/{id}"), "PUT", "/rooms/{id}", "PUT /rooms/{id}", true},
		{"delete", Delete("/rooms/{id}"), "DELETE", "/rooms/{id}", "DELETE /rooms/{id}", true},
		{"any", Any("/rooms/{id}"), "", "/rooms/{id}", "/rooms/{id}", true},
		{"root", Get("/"), "GET", "/", "GET /", false},
		{"lowercase method", New("post", "/x"), "POST", "/x", "POST /x", false},
		{"catch-all", Get("/files/{path...}"), "GET", "/files/{path...}", "GET /files/{path...}", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.route.Method(); got != test.method {
				t.Errorf("Method() = %q, want %q", got, test.method)
			}
			if got := test.route.Path(); got != test.path {
				t.Errorf("Path() = %q, want %q", got, test.path)
			}
			if got := test.route.Pattern(); got != test.pattern {
				t.Errorf("Pattern() = %q, want %q", got, test.pattern)
			}
			if got := test.route.HasParams(); got != test.params {
				t.Errorf("HasParams() = %v, want %v", got, test.params)
			}
		})
	}
}

func TestTrailingSlashNormalized(t *testing.T) {
	t.Parallel()
	if got := Get("/rooms/").Path(); got != "/rooms" {
		t.Fatalf("Path() = %q, want /rooms", got)
	}
}

func TestInvalidDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func()
	}{
		{"missing path", func() { New("GET", "") }},
		{"path without slash", func() { Get("rooms") }},
		{"query in path", func() { Get("/rooms?x=1") }},
		{"fragment in path", func() { Get("/rooms#top") }},
		{"method in path", func() { Get("GET /rooms") }},
		{"host qualified", func() { Get("//api.example.com/rooms") }},
		{"space in method", func() { New("GE T", "/rooms") }},
		{"unclosed placeholder", func() { Get("/rooms/{id") }},
		{"empty placeholder", func() { Get("/rooms/{}") }},
		{"invalid placeholder name", func() { Get("/rooms/{1id}") }},
		{"catch-all not last", func() { Get("/files/{path...}/x") }},
		{"duplicate placeholder", func() { Get("/rooms/{id}/{id}") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			test.build()
		})
	}
}

func TestHref(t *testing.T) {
	t.Parallel()

	room := Get("/rooms/{id}")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"plain", room.Href("42"), "/rooms/42"},
		{"space", room.Href("a b"), "/rooms/a%20b"},
		{"slash", room.Href("a/b"), "/rooms/a%2Fb"},
		{"question", room.Href("a?b"), "/rooms/a%3Fb"},
		{"hash", room.Href("a#b"), "/rooms/a%23b"},
		{"unicode", room.Href("pokój"), "/rooms/pok%C3%B3j"},
		{"static", Get("/rooms").Href(), "/rooms"},
		{"root", Get("/").Href(), "/"},
		{"catch-all", Get("/files/{path...}").Href("a/b c"), "/files/a/b%20c"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.got != test.want {
				t.Errorf("href = %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestHrefLiteralAndPlaceholder(t *testing.T) {
	t.Parallel()

	// Placeholders occupy whole segments, so a segment that mixes braces is
	// rejected at construction instead of being half-parsed.
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a mixed segment")
		}
	}()
	Get("/v{version}/x")
}

func TestHrefWrongValueCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func()
	}{
		{"too few", func() { Get("/rooms/{id}").Href() }},
		{"too many", func() { Get("/rooms/{id}").Href("42", "43") }},
		{"static with values", func() { Get("/rooms").Href("42") }},
		{"empty value", func() { Get("/rooms/{id}").Href("") }},
		{"dot segment", func() { Get("/rooms/{id}").Href("..") }},
		{"empty catch-all segment", func() { Get("/files/{path...}").Href("a//b") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			test.build()
		})
	}
}

func TestHrefQueryAndFragment(t *testing.T) {
	t.Parallel()

	got := Get("/rooms/{id}").
		URL("42").
		Query(url.Values{"b": {"2"}, "a": {"1", "3"}}).
		Fragment("a b#c").
		String()

	want := "/rooms/42?a=1&a=3&b=2#a%20b%23c"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestURLBuilderIsImmutable(t *testing.T) {
	t.Parallel()

	base := Get("/rooms").URL()
	withQuery := base.Query(url.Values{"a": {"1"}})
	withFragment := withQuery.Fragment("top")

	if got := base.String(); got != "/rooms" {
		t.Errorf("base changed: %q", got)
	}
	if got := withQuery.String(); got != "/rooms?a=1" {
		t.Errorf("query changed: %q", got)
	}
	if got := withFragment.String(); got != "/rooms?a=1#top" {
		t.Errorf("fragment = %q", got)
	}
	if got := withQuery.String(); got != "/rooms?a=1" {
		t.Errorf("fragment mutated the query URL: %q", got)
	}
}

func TestComparableAndUnexported(t *testing.T) {
	t.Parallel()

	if Get("/rooms/{id}") != Get("/rooms/{id}") {
		t.Fatal("identical routes do not compare equal")
	}
	if Get("/rooms") == Get("/rooms/{id}") {
		t.Fatal("different routes compare equal")
	}
	if Post("/rooms") == Get("/rooms") {
		t.Fatal("routes with different methods compare equal")
	}

	typ := reflect.TypeOf(Route{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			t.Fatalf("field %s is exported", typ.Field(i).Name)
		}
	}
}
