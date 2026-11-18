package datastar_test

import (
	"testing"
	"time"

	ds "github.com/matbur/stargo/datastar"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.InitOption
		want string
	}{
		{"plain", nil, ` data-init="@get(&#39;/live&#39;)"`},
		{"delay", []ds.InitOption{ds.WithInitDelay(250 * time.Millisecond)}, ` data-init__delay.250ms="x"`},
		{"view transition", []ds.InitOption{ds.WithInitViewTransition()}, ` data-init__viewtransition="x"`},
		{"both", []ds.InitOption{
			ds.WithInitViewTransition(),
			ds.WithInitDelay(time.Second),
		}, ` data-init__delay.1000ms__viewtransition="x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expression := "x"
			if tt.name == "plain" {
				expression = "@get('/live')"
			}
			if got := renderNode(t, ds.Init(expression, tt.opts...)); got != tt.want {
				t.Fatalf("Init() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInitPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.Init("x", ds.WithInitDelay(0)) })
	assertPanic(t, "datastar:", func() { ds.Init("x", ds.WithInitDelay(500*time.Microsecond)) })
}

func TestBind(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.BindOption
		want string
	}{
		{"plain", nil, ` data-bind="message"`},
		{"property", []ds.BindOption{ds.WithBindProp("checked")}, ` data-bind__prop.checked="done"`},
		{"event", []ds.BindOption{ds.WithBindEvent("change")}, ` data-bind__event.change="done"`},
		{"both", []ds.BindOption{ds.WithBindEvent("change"), ds.WithBindProp("checked")}, ` data-bind__prop.checked__event.change="done"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal := "message"
			if tt.name != "plain" {
				signal = "done"
			}
			if got := renderNode(t, ds.Bind(signal, tt.opts...)); got != tt.want {
				t.Fatalf("Bind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBindPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.Bind("") })
	assertPanic(t, "datastar:", func() { ds.Bind("a..b") })
	assertPanic(t, "datastar:", func() { ds.Bind("a b") })
	assertPanic(t, "datastar:", func() { ds.Bind("x", ds.WithBindProp("Bad Prop")) })
	assertPanic(t, "datastar:", func() { ds.Bind("x", ds.WithBindEvent("Bad Event")) })
}

func TestAttr(t *testing.T) {
	got := renderNode(t, ds.Attr("title", "$item.label"))
	want := ` data-attr:title="$item.label"`
	if got != want {
		t.Fatalf("Attr() = %q, want %q", got, want)
	}
}

func TestAttrEscapesExpression(t *testing.T) {
	got := renderNode(t, ds.Attr("title", `a"b'c&d<e>f`))
	want := ` data-attr:title="a&#34;b&#39;c&amp;d&lt;e&gt;f"`
	if got != want {
		t.Fatalf("Attr() = %q, want %q", got, want)
	}
}

func TestAttrPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.Attr("", "x") })
	assertPanic(t, "datastar:", func() { ds.Attr("Title", "x") })
	assertPanic(t, "datastar:", func() { ds.Attr("my title", "x") })
	assertPanic(t, "datastar:", func() { ds.Attr("my'key", "x") })
}

func TestReactiveValues(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"show", renderNode(t, ds.Show("$error != null")), ` data-show="$error != null"`},
		{"text", renderNode(t, ds.Text("$greeting")), ` data-text="$greeting"`},
		{"effect", renderNode(t, ds.Effect("el.focus()")), ` data-effect="el.focus()"`},
		{"class", renderNode(t, ds.Class("hidden", "!$expanded")), ` data-class:hidden="!$expanded"`},
		{"style", renderNode(t, ds.Style("width", "$pct + '%'")), ` data-style:width="$pct + &#39;%&#39;"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestReactiveValuesEscape(t *testing.T) {
	got := renderNode(t, ds.Show(`a"b'c&d<e>f`))
	want := ` data-show="a&#34;b&#39;c&amp;d&lt;e&gt;f"`
	if got != want {
		t.Fatalf("Show() = %q, want %q", got, want)
	}
}

func TestClassAndStylePanic(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.Class("", "x") })
	assertPanic(t, "datastar:", func() { ds.Class("Bad", "x") })
	assertPanic(t, "datastar:", func() { ds.Style("", "x") })
	assertPanic(t, "datastar:", func() { ds.Style("Bad Prop", "x") })
}

func TestComputed(t *testing.T) {
	got := renderNode(t, ds.Computed("fullName", "$first + $last"))
	want := ` data-computed="{&#39;fullName&#39;:() =&gt; ($first + $last)}"`
	if got != want {
		t.Fatalf("Computed() = %q, want %q", got, want)
	}
}

func TestComputedNested(t *testing.T) {
	got := renderNode(t, ds.Computed("ui.total", "$a + $b"))
	want := ` data-computed="{&#39;ui&#39;:{&#39;total&#39;:() =&gt; ($a + $b)}}"`
	if got != want {
		t.Fatalf("Computed() = %q, want %q", got, want)
	}
}

func TestComputedPanics(t *testing.T) {
	assertPanic(t, "datastar:", func() { ds.Computed("", "x") })
	assertPanic(t, "datastar:", func() { ds.Computed("a..b", "x") })
	assertPanic(t, "datastar:", func() { ds.Computed("a b", "x") })
}

func TestReactivePrefix(t *testing.T) {
	star := ds.New("data-star-")
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"init", renderNode(t, star.Init("x")), ` data-star-init="x"`},
		{"bind", renderNode(t, star.Bind("message")), ` data-star-bind="message"`},
		{"attr", renderNode(t, star.Attr("title", "x")), ` data-star-attr:title="x"`},
		{"signals", renderNode(t, star.Signals(map[string]any{"a": 1})), ` data-star-signals="{&#34;a&#34;:1}"`},
		{"computed", renderNode(t, star.Computed("total", "1")), ` data-star-computed="{&#39;total&#39;:() =&gt; (1)}"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
