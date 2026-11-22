package encoding_test

import (
	"testing"

	"github.com/matbur/stargo/internal/encoding"
)

func TestSelect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header    string
		supported []string
		want      string
	}{
		{"gzip", []string{"br", "gzip"}, "gzip"},
		{"GZIP", []string{"br", "gzip"}, "gzip"},
		{"x-gzip", []string{"br", "gzip"}, "gzip"},
		{"br", []string{"br", "gzip"}, "br"},
		{"br, gzip", []string{"br", "gzip"}, "br"},
		{"gzip;q=0.9, br;q=0.2", []string{"br", "gzip"}, "gzip"},
		{"gzip;q=0", []string{"br", "gzip"}, ""},
		{"br;q=0, gzip", []string{"br", "gzip"}, "gzip"},
		{"gzip;q=0, *;q=1", []string{"br", "gzip"}, "br"},
		{"*", []string{"br", "zstd", "gzip"}, "br"},
		{"*;q=0", []string{"br", "gzip"}, ""},
		{"identity", []string{"br", "gzip"}, ""},
		{"", []string{"br", "gzip"}, ""},
		{"gzip;q=bogus", []string{"br", "gzip"}, ""},
		{"gzip, *;q=0.1", []string{"br", "gzip"}, "gzip"},
		{"zstd", []string{"br", "gzip"}, ""},
	}
	for _, test := range cases {
		if got := encoding.Select(test.header, test.supported...); got != test.want {
			t.Errorf("Select(%q, %v) = %q, want %q", test.header, test.supported, got, test.want)
		}
	}
}
