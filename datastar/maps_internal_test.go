package datastar

import (
	"strings"
	"testing"
)

func TestEncodeJSObject(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]any
		want   string
	}{
		{"empty", map[string]any{}, `{}`},
		{"one", map[string]any{"open": "sidebarOpen"}, `{'open':sidebarOpen}`},
		{
			"several sorted",
			map[string]any{"title": "$label", "disabled": "$busy"},
			`{'disabled':$busy,'title':$label}`,
		},
		{
			"nested",
			map[string]any{"ui": map[string]any{"label": "$label", "open": "$open"}},
			`{'ui':{'label':$label,'open':$open}}`,
		},
		{
			"quoted key",
			map[string]any{"it's": "value"},
			`{'it\'s':value}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeJSObject(tt.values); got != tt.want {
				t.Fatalf("encodeJSObject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeJSObjectIsDeterministic(t *testing.T) {
	first := map[string]any{"a": "1", "b": "2", "c": "3"}
	second := map[string]any{"c": "3", "a": "1", "b": "2"}
	for i := 0; i < 20; i++ {
		if got, want := encodeJSObject(first), encodeJSObject(second); got != want {
			t.Fatalf("encodeJSObject() = %q, want %q", got, want)
		}
	}
}

func TestEncodeJSObjectPanicsOnUnsupportedValue(t *testing.T) {
	assertInternalPanic(t, "datastar:", func() {
		encodeJSObject(map[string]any{"count": 1})
	})
}

func TestEncodeJSObjectPanicsOnNULKey(t *testing.T) {
	assertInternalPanic(t, "datastar:", func() {
		encodeJSObject(map[string]any{"a\x00b": "value"})
	})
}

func TestStringValues(t *testing.T) {
	values := stringValues(map[string]string{"open": "$open"})
	if values["open"] != "$open" || len(values) != 1 {
		t.Fatalf("stringValues() = %v", values)
	}
	if encoded := encodeJSObject(values); strings.Contains(encoded, "&#34;") {
		t.Fatalf("encodeJSObject() = %q, want unescaped source", encoded)
	}
}

func assertInternalPanic(t *testing.T, messagePrefix string, fn func()) {
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
