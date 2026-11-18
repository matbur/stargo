package datastar

import (
	"io"
	"strings"
	"testing"
)

func render(t *testing.T, node interface{ Render(io.Writer) error }) string {
	t.Helper()
	var b strings.Builder
	if err := node.Render(&b); err != nil {
		t.Fatalf("render: %v", err)
	}
	return b.String()
}

func TestBareAttributeRendersNameOnly(t *testing.T) {
	got := render(t, bareAttribute("data-ignore"))
	if got != " data-ignore" {
		t.Fatalf("bare attribute = %q, want %q", got, " data-ignore")
	}
}

func TestValuedAttributeKeepsAssignment(t *testing.T) {
	attribute := Attribute{name: "data-show", value: `$open && "x"`}
	got := render(t, attribute)
	if !strings.HasPrefix(got, ` data-show="`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("valued attribute = %q, want a quoted value", got)
	}
}

func TestBareAttributeKeepsNameVerbatim(t *testing.T) {
	got := render(t, bareAttribute("data-ignore-morph"))
	if strings.Contains(got, "=") {
		t.Fatalf("bare attribute = %q, want no assignment", got)
	}
}
