package datastar

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandSignalPaths(t *testing.T) {
	got := expandSignalPaths(map[string]any{
		"count":   0,
		"ui.open": true,
		"a.b.c":   "x",
	})
	want := map[string]any{
		"count": 0,
		"ui":    map[string]any{"open": true},
		"a":     map[string]any{"b": map[string]any{"c": "x"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandSignalPaths() = %#v, want %#v", got, want)
	}
}

func TestExpandSignalPathsEmpty(t *testing.T) {
	got := expandSignalPaths(map[string]any{})
	if len(got) != 0 {
		t.Fatalf("expandSignalPaths() = %#v, want empty map", got)
	}
}

func TestEncodeSignalsDeterministic(t *testing.T) {
	first := encodeSignals(map[string]any{"open": false, "count": 0})
	second := encodeSignals(map[string]any{"count": 0, "open": false})
	if first != second {
		t.Fatalf("encodeSignals() = %q and %q, want identical", first, second)
	}
	if first != `{"count":0,"open":false}` {
		t.Fatalf("encodeSignals() = %q", first)
	}
}

func TestExpandSignalPathsPanics(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{"leaf then path", map[string]any{"ui": true, "ui.open": false}},
		{"path then leaf", map[string]any{"ui.open": false, "ui": true}},
		{"object then path", map[string]any{
			"ui":      map[string]any{"open": true},
			"ui.open": false,
		}},
		{"empty segment", map[string]any{"ui..open": true}},
		{"leading dot", map[string]any{".ui": true}},
		{"trailing dot", map[string]any{"ui.": true}},
		{"empty name", map[string]any{"": true}},
		{"unsupported value", map[string]any{"bad": make(chan int)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := testPanic(t, func() { encodeSignals(tt.payload) })
			if !strings.HasPrefix(message, "datastar:") {
				t.Fatalf("panic message %q lacks datastar: prefix", message)
			}
		})
	}
}
