package common_test

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/matbur/stargo/examples/chat-room/app/common"
)

func TestPageShell(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	if err := common.Page(h.Main(g.Text("feature body"))).Render(&builder); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := builder.String()

	checks := []string{
		"<!doctype html>",
		`<html lang="en">`,
		`<title>Stario Chat</title>`,
		"<style>",
		".lobby-container",
		"<main>feature body</main>",
		"v1.0.4",
		`type="module"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Fatalf("shell missing %q", want)
		}
	}

	if strings.Contains(html, "<link") {
		t.Fatal("shell loads an external stylesheet")
	}
}
