package datastar_test

import (
	"testing"

	ds "github.com/matbur/stargo/datastar"
)

func TestSignals(t *testing.T) {
	got := renderNode(t, ds.Signals(map[string]any{"count": 0, "open": false}))
	want := ` data-signals="{&#34;count&#34;:0,&#34;open&#34;:false}"`
	if got != want {
		t.Fatalf("Signals() = %q, want %q", got, want)
	}
}

func TestSignalsIfMissing(t *testing.T) {
	got := renderNode(t, ds.Signals(map[string]any{"count": 0}, ds.WithIfMissing()))
	want := ` data-signals__ifmissing="{&#34;count&#34;:0}"`
	if got != want {
		t.Fatalf("Signals() = %q, want %q", got, want)
	}
}

func TestSignalsEscapesValues(t *testing.T) {
	got := renderNode(t, ds.Signals(map[string]any{"text": "a'b&c"}))
	want := ` data-signals="{&#34;text&#34;:&#34;a&#39;b&amp;c&#34;}"`
	if got != want {
		t.Fatalf("Signals() = %q, want %q", got, want)
	}
}

func TestSignalsEmptyPayload(t *testing.T) {
	got := renderNode(t, ds.Signals(map[string]any{}))
	want := ` data-signals="{}"`
	if got != want {
		t.Fatalf("Signals() = %q, want %q", got, want)
	}
}

func TestSignalsNestedPath(t *testing.T) {
	got := renderNode(t, ds.Signals(map[string]any{"ui.open": true}))
	want := ` data-signals="{&#34;ui&#34;:{&#34;open&#34;:true}}"`
	if got != want {
		t.Fatalf("Signals() = %q, want %q", got, want)
	}
}

func TestSignalsInstanceParity(t *testing.T) {
	payload := map[string]any{"count": 0}
	fromPackage := renderNode(t, ds.Signals(payload, ds.WithIfMissing()))
	fromInstance := renderNode(t, ds.Default().Signals(payload, ds.WithIfMissing()))
	if fromPackage != fromInstance {
		t.Fatalf("package = %q, instance = %q", fromPackage, fromInstance)
	}
}
