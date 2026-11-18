package datastar

import (
	"strings"
	"testing"
)

func TestSignalFilterObject(t *testing.T) {
	tests := []struct {
		name  string
		build func(*signalFilters)
		want  string
		found bool
	}{
		{"none", func(*signalFilters) {}, "", false},
		{"include", func(f *signalFilters) { f.setInclude("draft") }, `{'include':'draft'}`, true},
		{"exclude", func(f *signalFilters) { f.setExclude("secret") }, `{'exclude':'secret'}`, true},
		{
			"both",
			func(f *signalFilters) {
				f.setExclude("secret")
				f.setInclude("draft")
			},
			`{'include':'draft','exclude':'secret'}`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filters signalFilters
			tt.build(&filters)
			got, found := filters.object()
			if got != tt.want || found != tt.found {
				t.Fatalf("object() = %q, %v, want %q, %v", got, found, tt.want, tt.found)
			}
		})
	}
}

func TestSignalFilterPatternsPanic(t *testing.T) {
	invalid := []string{"", "a\x00b", "a\nb", "a/b"}
	for _, pattern := range invalid {
		t.Run(strings.ReplaceAll(pattern, "\x00", "NUL"), func(t *testing.T) {
			assertInternalPanic(t, "datastar:", func() {
				var filters signalFilters
				filters.setInclude(pattern)
			})
			assertInternalPanic(t, "datastar:", func() {
				var filters signalFilters
				filters.setExclude(pattern)
			})
		})
	}

	var filters signalFilters
	filters.setInclude(`a\/b`)
	if got, _ := filters.object(); got != `{'include':'a\\/b'}` {
		t.Fatalf("escaped slash object() = %q", got)
	}
}
