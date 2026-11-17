package stargo

import (
	"bytes"
	"os"
	"testing"

	"github.com/matbur/stargo/internal/console"
)

func TestConsoleEnv(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"", "auto"},
		{"auto", "auto"},
		{"0", "off"},
		{"off", "off"},
		{"false", "off"},
		{"1", "on"},
		{"true", "on"},
		{"on", "on"},
	}
	for _, test := range tests {
		t.Setenv("STARGO_CONSOLE", test.value)
		if got := consoleEnv(); got != test.want {
			t.Errorf("consoleEnv() for %q = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestResolveConsoleFromEnvironment(t *testing.T) {
	t.Setenv("STARGO_CONSOLE", "0")
	if got := resolveConsole(nil); got != nil {
		t.Fatal("STARGO_CONSOLE=0 did not disable console output")
	}

	t.Setenv("STARGO_CONSOLE", "1")
	if got := resolveConsole(nil); got == nil {
		t.Fatal("STARGO_CONSOLE=1 did not enable console output")
	}

	t.Setenv("STARGO_CONSOLE", "")
	if isTerminal(os.Stdout) {
		t.Skip("standard output is a terminal; auto detection cannot be tested")
	}
	if got := resolveConsole(nil); got != nil {
		t.Fatal("auto detection enabled console output for a non-terminal")
	}
}

func TestResolveConsoleOptionOverridesEnvironment(t *testing.T) {
	t.Setenv("STARGO_CONSOLE", "0")

	var buffer bytes.Buffer
	writer := resolveConsole(&consoleTarget{writer: &buffer, set: true})
	if writer == nil {
		t.Fatal("Console(writer) was suppressed by the environment")
	}
	writer.Write(consoleBlock())
	if buffer.Len() == 0 {
		t.Fatal("console writer produced no output")
	}

	if got := resolveConsole(&consoleTarget{set: true}); got != nil {
		t.Fatal("NoConsole() did not disable output")
	}
}

func consoleBlock() console.Block {
	return console.Block{Name: "test"}
}

func TestConsoleWidth(t *testing.T) {
	t.Setenv("COLUMNS", "50")
	if got := consoleWidth(); got != 49 {
		t.Fatalf("consoleWidth() = %d, want 49", got)
	}

	t.Setenv("COLUMNS", "200")
	if got := consoleWidth(); got != 100 {
		t.Fatalf("consoleWidth() = %d, want 100", got)
	}

	t.Setenv("COLUMNS", "invalid")
	if got := consoleWidth(); got != 100 {
		t.Fatalf("consoleWidth() = %d, want 100", got)
	}
}

func TestColorDisabledForNonTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var buffer bytes.Buffer
	writer := resolveConsole(&consoleTarget{writer: &buffer, set: true})
	writer.Write(consoleBlock())
	if bytes.Contains(buffer.Bytes(), []byte("\x1b[")) {
		t.Fatalf("non-terminal output contains ANSI: %q", buffer.String())
	}
}
