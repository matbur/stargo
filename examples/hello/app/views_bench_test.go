package app_test

import (
	"io"
	"testing"

	"github.com/matbur/stargo/examples/hello/app"
)

// BenchmarkPage measures the counter page exactly as the example builds it.
func BenchmarkPage(b *testing.B) {
	page := app.Page(42, app.Live.Href())
	b.Run("render_prebuilt", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if err := page.Render(io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("build_and_render", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if err := app.Page(42, app.Live.Href()).Render(io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCounterRender(b *testing.B) {
	counter := app.Counter(42)
	b.ReportAllocs()
	for range b.N {
		if err := counter.Render(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
