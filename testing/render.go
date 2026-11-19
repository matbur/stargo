package stargotest

import (
	"context"

	"github.com/matbur/stargo/internal/render"
)

// Render renders value with the same renderer that responses and patches use.
//
// Accepted values are a context renderer, a writer renderer, a string, and a
// byte slice. A render failure fails the test.
func Render(tb TB, value any) string {
	tb.Helper()

	html, err := render.HTML(context.Background(), value)
	if err != nil {
		tb.Fatalf("stargotest: render HTML: %v", err)
		return ""
	}
	return string(html)
}
