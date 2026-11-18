package datastar_test

import (
	"testing"

	ds "github.com/matbur/stargo/datastar"
)

func TestAttrs(t *testing.T) {
	tests := []struct {
		name    string
		mapping map[string]string
		want    string
	}{
		{
			name:    "one",
			mapping: map[string]string{"open": "sidebarOpen"},
			want:    ` data-attr="{&#39;open&#39;:sidebarOpen}"`,
		},
		{
			name:    "several sorted",
			mapping: map[string]string{"title": "$label", "disabled": "$busy"},
			want:    ` data-attr="{&#39;disabled&#39;:$busy,&#39;title&#39;:$label}"`,
		},
		{
			name:    "empty",
			mapping: map[string]string{},
			want:    ` data-attr="{}"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.Attrs(tt.mapping)); got != tt.want {
				t.Fatalf("Attrs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttrsPrefix(t *testing.T) {
	got := renderNode(t, ds.New("data-star-").Attrs(map[string]string{"open": "$open"}))
	want := ` data-star-attr="{&#39;open&#39;:$open}"`
	if got != want {
		t.Fatalf("Attrs() = %q, want %q", got, want)
	}
}

func TestAttrsPanics(t *testing.T) {
	invalid := []struct {
		name    string
		mapping map[string]string
	}{
		{"uppercase", map[string]string{"Title": "value"}},
		{"space", map[string]string{"with space": "value"}},
		{"empty", map[string]string{"": "value"}},
		{"quote", map[string]string{`with"quote`: "value"}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			assertPanic(t, "datastar:", func() { ds.Attrs(tt.mapping) })
		})
	}
}

func TestClasses(t *testing.T) {
	got := renderNode(t, ds.Classes(map[string]string{
		"loading": "$pending",
		"hidden":  "!$open",
	}))
	want := ` data-class="{&#39;hidden&#39;:!$open,&#39;loading&#39;:$pending}"`
	if got != want {
		t.Fatalf("Classes() = %q, want %q", got, want)
	}
}

func TestClassesPanicsOnInvalidName(t *testing.T) {
	assertPanic(t, "datastar:", func() {
		ds.Classes(map[string]string{"Bad Name": "$x"})
	})
}

func TestStyles(t *testing.T) {
	got := renderNode(t, ds.Styles(map[string]string{
		"width":   "$pct + '%'",
		"opacity": "$fade",
	}))
	want := ` data-style="{&#39;opacity&#39;:$fade,&#39;width&#39;:$pct + &#39;%&#39;}"`
	if got != want {
		t.Fatalf("Styles() = %q, want %q", got, want)
	}
}

func TestStylesPanicsOnInvalidProperty(t *testing.T) {
	assertPanic(t, "datastar:", func() {
		ds.Styles(map[string]string{"": "$x"})
	})
}

func TestComputeds(t *testing.T) {
	got := renderNode(t, ds.Computeds(map[string]string{
		"total": "$a + $b",
		"ready": "$count > 0",
	}))
	want := ` data-computed="{&#39;ready&#39;:() =&gt; ($count &gt; 0),&#39;total&#39;:() =&gt; ($a + $b)}"`
	if got != want {
		t.Fatalf("Computeds() = %q, want %q", got, want)
	}
}

func TestComputedsNested(t *testing.T) {
	got := renderNode(t, ds.Computeds(map[string]string{"ui.total": "$a + $b"}))
	want := ` data-computed="{&#39;ui&#39;:{&#39;total&#39;:() =&gt; ($a + $b)}}"`
	if got != want {
		t.Fatalf("Computeds() = %q, want %q", got, want)
	}
}

func TestComputedsPanicsOnConflictingPaths(t *testing.T) {
	assertPanic(t, "datastar: conflicting signal paths", func() {
		ds.Computeds(map[string]string{"ui": "$a", "ui.total": "$b"})
	})
}

func TestComputedsPanicsOnEmptySegment(t *testing.T) {
	assertPanic(t, "datastar:", func() {
		ds.Computeds(map[string]string{"ui..total": "$a"})
	})
}

func TestIgnore(t *testing.T) {
	tests := []struct {
		name string
		got  ds.Attribute
		want string
	}{
		{"plain", ds.Ignore(), ` data-ignore`},
		{"self", ds.Ignore(ds.WithIgnoreSelf()), ` data-ignore__self`},
		{"morph", ds.IgnoreMorph(), ` data-ignore-morph`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, tt.got); got != tt.want {
				t.Fatalf("render = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIgnorePrefix(t *testing.T) {
	star := ds.New("data-star-")
	if got := renderNode(t, star.Ignore()); got != " data-star-ignore" {
		t.Fatalf("Ignore() = %q", got)
	}
	if got := renderNode(t, star.IgnoreMorph()); got != " data-star-ignore-morph" {
		t.Fatalf("IgnoreMorph() = %q", got)
	}
}

func TestIndicator(t *testing.T) {
	if got := renderNode(t, ds.Indicator("saving")); got != ` data-indicator="saving"` {
		t.Fatalf("Indicator() = %q", got)
	}
	assertPanic(t, "datastar:", func() { ds.Indicator("") })
	assertPanic(t, "datastar:", func() { ds.Indicator("ui..saving") })
}

func TestRef(t *testing.T) {
	if got := renderNode(t, ds.Ref("searchInput")); got != ` data-ref="searchInput"` {
		t.Fatalf("Ref() = %q", got)
	}
	assertPanic(t, "datastar:", func() { ds.Ref("") })
	assertPanic(t, "datastar:", func() { ds.Ref("ui..input") })
}

func TestPersist(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.PersistOption
		want string
	}{
		{"plain", nil, ` data-persist`},
		{
			"keyed session with filters",
			[]ds.PersistOption{
				ds.WithPersistKey("prefs"),
				ds.WithPersistSession(),
				ds.WithPersistInclude("draft"),
			},
			` data-persist:prefs__session="{&#39;include&#39;:&#39;draft&#39;}"`,
		},
		{
			"exclude only",
			[]ds.PersistOption{ds.WithPersistExclude("secret")},
			` data-persist="{&#39;exclude&#39;:&#39;secret&#39;}"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.Persist(tt.opts...)); got != tt.want {
				t.Fatalf("Persist() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPersistPanics(t *testing.T) {
	cases := map[string]func(){
		"empty key":     func() { ds.Persist(ds.WithPersistKey("")) },
		"uppercase key": func() { ds.Persist(ds.WithPersistKey("Prefs")) },
		"invalid include": func() {
			ds.Persist(ds.WithPersistInclude("a/b"))
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			assertPanic(t, "datastar:", fn)
		})
	}
}

func TestPreserveAttr(t *testing.T) {
	if got := renderNode(t, ds.PreserveAttr("data-testid", "id")); got != ` data-preserve-attr="data-testid id"` {
		t.Fatalf("PreserveAttr() = %q", got)
	}
	if got := renderNode(t, ds.PreserveAttr("viewBox", "xlink:href")); got != ` data-preserve-attr="viewBox xlink:href"` {
		t.Fatalf("PreserveAttr() = %q", got)
	}
}

func TestPreserveAttrPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.PreserveAttr() })
	assertPanic(t, "datastar:", func() { ds.PreserveAttr("") })
	for _, name := range []string{"a b", "a\tb", "a\nb", `a"b`, "a'b", "a=b", "a<b", "a>b", "a/b", "a&b", "a\x00b", "a\x7fb"} {
		name := name
		t.Run(name, func(t *testing.T) {
			assertPanic(t, "datastar:", func() { ds.PreserveAttr(name) })
		})
	}
}

func TestJSONSignals(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.JSONSignalsOption
		want string
	}{
		{"plain", nil, ` data-json-signals`},
		{"filtered", []ds.JSONSignalsOption{ds.WithJSONSignalsInclude("email|password")}, ` data-json-signals="{&#39;include&#39;:&#39;email|password&#39;}"`},
		{"terse", []ds.JSONSignalsOption{ds.WithJSONSignalsTerse()}, ` data-json-signals__terse`},
		{
			"terse filtered",
			[]ds.JSONSignalsOption{ds.WithJSONSignalsTerse(), ds.WithJSONSignalsExclude("secret")},
			` data-json-signals__terse="{&#39;exclude&#39;:&#39;secret&#39;}"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.JSONSignals(tt.opts...)); got != tt.want {
				t.Fatalf("JSONSignals() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONSignalsPrefix(t *testing.T) {
	got := renderNode(t, ds.New("data-star-").JSONSignals(ds.WithJSONSignalsTerse()))
	if got != " data-star-json-signals__terse" {
		t.Fatalf("JSONSignals() = %q", got)
	}
}

func TestJSONSignalsPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.JSONSignals(ds.WithJSONSignalsInclude("a/b")) })
}
