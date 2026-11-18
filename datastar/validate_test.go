package datastar

import (
	"strings"
	"testing"
)

func testPanic(t *testing.T, fn func()) string {
	t.Helper()
	var message string
	func() {
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatalf("expected panic")
			}
			text, ok := recovered.(string)
			if !ok {
				t.Fatalf("panic value is %T, want string", recovered)
			}
			message = text
		}()
		fn()
	}()
	return message
}

func TestValidateAttributeKey(t *testing.T) {
	for _, key := range []string{"title", "aria-label", "data-x", "x1", "a-b-c"} {
		validateAttributeKey(key)
	}

	invalid := []string{
		"",
		"Title",
		"my title",
		"my'key",
		`my"key`,
		"my/key",
		"my=key",
		"-title",
		"title-",
		"my_key",
		"kluczé",
	}
	for _, key := range invalid {
		t.Run(key, func(t *testing.T) {
			message := testPanic(t, func() { validateAttributeKey(key) })
			if !strings.HasPrefix(message, "datastar:") {
				t.Fatalf("panic message %q lacks datastar: prefix", message)
			}
		})
	}
}

func TestValidateSignalName(t *testing.T) {
	for _, name := range []string{"message", "ui.open", "isChecked", "_local", "room-title", "a1.b2.c3"} {
		validateSignalName(name)
	}

	invalid := []string{
		"",
		".open",
		"ui.",
		"ui..open",
		"my signal",
		"my'name",
		`my"name`,
		"my/name",
		"my=name",
		"signalé",
	}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			message := testPanic(t, func() { validateSignalName(name) })
			if !strings.HasPrefix(message, "datastar:") {
				t.Fatalf("panic message %q lacks datastar: prefix", message)
			}
		})
	}
}
