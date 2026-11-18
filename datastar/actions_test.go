package datastar_test

import (
	"math"
	"testing"
	"time"

	ds "github.com/matbur/stargo/datastar"
)

func TestActionVerbs(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"get", ds.Get("/x"), `@get('/x')`},
		{"post", ds.Post("/x"), `@post('/x')`},
		{"put", ds.Put("/x"), `@put('/x')`},
		{"patch", ds.Patch("/x"), `@patch('/x')`},
		{"delete", ds.Delete("/x"), `@delete('/x')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestActionPathQuoting(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"quote", "/a'b", `@get('/a\'b')`},
		{"backslash", `/a\b`, `@get('/a\\b')`},
		{"newline", "/a\nb", `@get('/a\nb')`},
		{"carriage return", "/a\rb", `@get('/a\rb')`},
		{"line separator", "/a\u2028b", `@get('/a\u2028b')`},
		{"paragraph separator", "/a\u2029b", `@get('/a\u2029b')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ds.Get(tt.path); got != tt.want {
				t.Fatalf("Get(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}

	assertPanic(t, "datastar:", func() { ds.Get("/a\x00b") })
}

func TestActionOptions(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.ActionOption
		want string
	}{
		{
			name: "json content type",
			opts: []ds.ActionOption{ds.WithContentType(ds.ContentTypeJSON)},
			want: `@post('/x', {contentType: 'json'})`,
		},
		{
			name: "form content type",
			opts: []ds.ActionOption{ds.WithForm()},
			want: `@post('/x', {contentType: 'form'})`,
		},
		{
			name: "selector",
			opts: []ds.ActionOption{ds.WithForm(), ds.WithSelector("#form")},
			want: `@post('/x', {contentType: 'form', selector: '#form'})`,
		},
		{
			name: "headers",
			opts: []ds.ActionOption{ds.WithHeaders(map[string]string{"X-Trace": "abc"})},
			want: `@post('/x', {headers: {'X-Trace': 'abc'}})`,
		},
		{
			name: "open when hidden",
			opts: []ds.ActionOption{ds.WithOpenWhenHidden(true)},
			want: `@post('/x', {openWhenHidden: true})`,
		},
		{
			name: "payload",
			opts: []ds.ActionOption{ds.WithPayload(map[string]string{"q": "$query"})},
			want: `@post('/x', {payload: {q: $query}})`,
		},
		{
			name: "include filter",
			opts: []ds.ActionOption{ds.WithSignalsInclude("^a")},
			want: `@post('/x', {filterSignals: {include: /^a/}})`,
		},
		{
			name: "exclude filter",
			opts: []ds.ActionOption{ds.WithSignalsExclude("b$")},
			want: `@post('/x', {filterSignals: {exclude: /b$/}})`,
		},
		{
			name: "retry mode",
			opts: []ds.ActionOption{ds.WithRetry(ds.RetryAlways)},
			want: `@post('/x', {retry: 'always'})`,
		},
		{
			name: "retry interval",
			opts: []ds.ActionOption{ds.WithRetryInterval(2 * time.Second)},
			want: `@post('/x', {retryInterval: 2000})`,
		},
		{
			name: "retry scaler",
			opts: []ds.ActionOption{ds.WithRetryScaler(1.5)},
			want: `@post('/x', {retryScaler: 1.5})`,
		},
		{
			name: "retry max wait",
			opts: []ds.ActionOption{ds.WithRetryMaxWait(30 * time.Second)},
			want: `@post('/x', {retryMaxWait: 30000})`,
		},
		{
			name: "retry max count",
			opts: []ds.ActionOption{ds.WithRetryMaxCount(5)},
			want: `@post('/x', {retryMaxCount: 5})`,
		},
		{
			name: "request cancellation",
			opts: []ds.ActionOption{ds.WithRequestCancellation(ds.CancellationDisabled)},
			want: `@post('/x', {requestCancellation: 'disabled'})`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ds.Post("/x", tt.opts...); got != tt.want {
				t.Fatalf("Post() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionOptionOrder(t *testing.T) {
	first := ds.Post("/rooms",
		ds.WithForm(),
		ds.WithSelector("#create-room"),
		ds.WithHeaders(map[string]string{"B": "2", "A": "1"}),
		ds.WithOpenWhenHidden(true),
		ds.WithPayload(map[string]string{"q": "$query"}),
		ds.WithRetry(ds.RetryAlways),
		ds.WithRetryInterval(2*time.Second),
		ds.WithRequestCancellation(ds.CancellationAuto),
	)
	second := ds.Post("/rooms",
		ds.WithRequestCancellation(ds.CancellationAuto),
		ds.WithRetryInterval(2*time.Second),
		ds.WithRetry(ds.RetryAlways),
		ds.WithPayload(map[string]string{"q": "$query"}),
		ds.WithOpenWhenHidden(true),
		ds.WithHeaders(map[string]string{"A": "1", "B": "2"}),
		ds.WithSelector("#create-room"),
		ds.WithForm(),
	)
	want := `@post('/rooms', {contentType: 'form', selector: '#create-room', ` +
		`headers: {'A': '1', 'B': '2'}, openWhenHidden: true, payload: {q: $query}, ` +
		`retry: 'always', retryInterval: 2000, requestCancellation: 'auto'})`
	if first != want {
		t.Fatalf("Post() = %q, want %q", first, want)
	}
	if second != want {
		t.Fatalf("reordered Post() = %q, want %q", second, want)
	}
}

func TestActionPanics(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"unknown content type", func() {
			ds.Post("/x", ds.WithContentType(ds.ContentType("xml")))
		}},
		{"selector without form", func() {
			ds.Post("/x", ds.WithSelector("#form"))
		}},
		{"empty selector", func() {
			ds.Post("/x", ds.WithForm(), ds.WithSelector(""))
		}},
		{"empty header name", func() {
			ds.Post("/x", ds.WithHeaders(map[string]string{"": "abc"}))
		}},
		{"invalid header name", func() {
			ds.Post("/x", ds.WithHeaders(map[string]string{"Bad Header": "abc"}))
		}},
		{"zero retry interval", func() {
			ds.Post("/x", ds.WithRetryInterval(0))
		}},
		{"negative retry max wait", func() {
			ds.Post("/x", ds.WithRetryMaxWait(-time.Second))
		}},
		{"nan retry scaler", func() {
			ds.Post("/x", ds.WithRetryScaler(math.NaN()))
		}},
		{"zero retry scaler", func() {
			ds.Post("/x", ds.WithRetryScaler(0))
		}},
		{"negative retry count", func() {
			ds.Post("/x", ds.WithRetryMaxCount(-1))
		}},
		{"unknown retry mode", func() {
			ds.Post("/x", ds.WithRetry(ds.RetryMode("sometimes")))
		}},
		{"unknown cancellation mode", func() {
			ds.Post("/x", ds.WithRequestCancellation(ds.CancellationMode("never")))
		}},
		{"empty include pattern", func() {
			ds.Post("/x", ds.WithSignalsInclude(""))
		}},
		{"unescaped slash", func() {
			ds.Post("/x", ds.WithSignalsInclude("a/b"))
		}},
		{"newline in pattern", func() {
			ds.Post("/x", ds.WithSignalsExclude("a\nb"))
		}},
		{"invalid payload key", func() {
			ds.Post("/x", ds.WithPayload(map[string]string{"a b": "$x"}))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPanic(t, "datastar:", tt.fn)
		})
	}
}

func TestActionFilters(t *testing.T) {
	got := ds.Post("/x", ds.WithSignalsInclude("^a"), ds.WithSignalsExclude("b$"))
	want := `@post('/x', {filterSignals: {include: /^a/, exclude: /b$/}})`
	if got != want {
		t.Fatalf("Post() = %q, want %q", got, want)
	}
}

func TestActionComposesWithOn(t *testing.T) {
	got := renderNode(t, ds.On("click", ds.Post("/command")))
	want := ` data-on:click="@post(&#39;/command&#39;)"`
	if got != want {
		t.Fatalf("On(Post()) = %q, want %q", got, want)
	}
}
