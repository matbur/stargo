package datastar_test

import (
	"io"
	"strings"
	"testing"
	"time"

	g "maragu.dev/gomponents"

	ds "github.com/matbur/stargo/datastar"
)

func renderNode(t *testing.T, node interface{ Render(io.Writer) error }) string {
	t.Helper()
	var b strings.Builder
	if err := node.Render(&b); err != nil {
		t.Fatalf("render: %v", err)
	}
	return b.String()
}

func assertPanic(t *testing.T, messagePrefix string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic with prefix %q", messagePrefix)
		}
		message, ok := recovered.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", recovered)
		}
		if !strings.HasPrefix(message, messagePrefix) {
			t.Fatalf("panic message %q does not start with %q", message, messagePrefix)
		}
	}()
	fn()
}

func TestOn(t *testing.T) {
	got := renderNode(t, ds.On("click", "@post('/command')"))
	want := ` data-on:click="@post(&#39;/command&#39;)"`
	if got != want {
		t.Fatalf("On() = %q, want %q", got, want)
	}
}

func TestOnEscapesAttributeValues(t *testing.T) {
	values := []string{
		"\x00",
		`"`,
		`'`,
		"&",
		"<",
		">",
		`<a href="x">'&</a>`,
		"plain text",
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			got := renderNode(t, ds.On("click", value))
			want := renderNode(t, g.Attr("data-on:click", value))
			if got != want {
				t.Fatalf("On() = %q, gomponents = %q", got, want)
			}
		})
	}
}

func TestOnModifiers(t *testing.T) {
	tests := []struct {
		name string
		opt  ds.OnOption
		want string
	}{
		{"once", ds.WithOnce(), "data-on:click__once"},
		{"passive", ds.WithPassive(), "data-on:click__passive"},
		{"capture", ds.WithCapture(), "data-on:click__capture"},
		{"stop", ds.WithStop(), "data-on:click__stop"},
		{"prevent", ds.WithPrevent(), "data-on:click__prevent"},
		{"window", ds.WithWindow(), "data-on:click__window"},
		{"document", ds.WithDocument(), "data-on:click__document"},
		{"outside", ds.WithOutside(), "data-on:click__outside"},
		{"delay", ds.WithDelay(250 * time.Millisecond), "data-on:click__delay.250ms"},
		{"debounce", ds.WithDebounce(500 * time.Millisecond), "data-on:click__debounce.500ms"},
		{"throttle", ds.WithThrottle(time.Second), "data-on:click__throttle.1000ms"},
		{"view transition", ds.WithViewTransition(), "data-on:click__viewtransition"},
		{"case camel", ds.WithCase(ds.CaseCamel), "data-on:click__case.camel"},
		{"case kebab", ds.WithCase(ds.CaseKebab), "data-on:click"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderNode(t, ds.On("click", "x", tt.opt))
			want := " " + tt.want + `="x"`
			if got != want {
				t.Fatalf("On() = %q, want %q", got, want)
			}
		})
	}
}

func TestOnModifierOrderAndDuplicates(t *testing.T) {
	first := renderNode(t, ds.On("submit", "x",
		ds.WithThrottle(300*time.Millisecond),
		ds.WithPrevent(),
		ds.WithOnce(),
	))
	second := renderNode(t, ds.On("submit", "x",
		ds.WithOnce(),
		ds.WithPrevent(),
		ds.WithOnce(),
		ds.WithThrottle(300*time.Millisecond),
	))
	want := ` data-on:submit__once__prevent__throttle.300ms="x"`
	if first != want {
		t.Fatalf("On() = %q, want %q", first, want)
	}
	if second != want {
		t.Fatalf("reordered On() = %q, want %q", second, want)
	}
}

func TestOnPanics(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"passive with prevent", func() {
			ds.On("click", "x", ds.WithPassive(), ds.WithPrevent())
		}},
		{"unknown case", func() {
			ds.On("click", "x", ds.WithCase(ds.Case("upper")))
		}},
		{"zero delay", func() {
			ds.On("click", "x", ds.WithDelay(0))
		}},
		{"negative debounce", func() {
			ds.On("click", "x", ds.WithDebounce(-time.Second))
		}},
		{"zero throttle", func() {
			ds.On("click", "x", ds.WithThrottle(0))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPanic(t, "datastar:", tt.fn)
		})
	}
}

func TestEventNames(t *testing.T) {
	got := renderNode(t, ds.On("my-event", "x"))
	if got != ` data-on:my-event="x"` {
		t.Fatalf("On() = %q", got)
	}

	invalid := []string{
		"",
		"myEvent",
		"my event",
		"my'event",
		"my>event",
		"my/event",
		"my=event",
		"-myevent",
		"myevent-",
		"my_event",
		"myeventé",
	}
	for _, event := range invalid {
		t.Run(event, func(t *testing.T) {
			assertPanic(t, "datastar:", func() { ds.On(event, "x") })
		})
	}
}

func TestPrefixes(t *testing.T) {
	if got := renderNode(t, ds.Default().On("click", "x")); got != ` data-on:click="x"` {
		t.Fatalf("default On() = %q", got)
	}
	if got := renderNode(t, ds.New("data-star-").On("click", "x")); got != ` data-star-on:click="x"` {
		t.Fatalf("aliased On() = %q", got)
	}
	if got := renderNode(t, ds.New("data-star").On("click", "x")); got != ` data-star-on:click="x"` {
		t.Fatalf("normalized On() = %q", got)
	}

	invalid := []string{
		"",
		"star-",
		"data--",
		"data-STAR-",
		"data-bad alias-",
		"data-star_",
	}
	for _, prefix := range invalid {
		t.Run(prefix, func(t *testing.T) {
			assertPanic(t, "datastar:", func() { ds.New(prefix) })
		})
	}
}
