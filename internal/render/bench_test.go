package render_test

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/matbur/stargo/examples/hello/app"
	"github.com/matbur/stargo/internal/render"
)

// benchRows matches the 50-row table used by Stario's page benchmark.
const benchRows = 50

// contextNode adapts a gomponents tree so it exercises the context renderer
// branch instead of the writer renderer branch.
type contextNode struct {
	node g.Node
}

func (n contextNode) Render(_ context.Context, w io.Writer) error {
	return n.node.Render(w)
}

func BenchmarkHTML(b *testing.B) {
	// The same static tree as Stario's microbench suite.
	small := h.Div(
		g.Attr("class", "wrap p-4"),
		h.Span(g.Attr("class", "name"), g.Text("Item")),
		h.Span(g.Attr("class", "price"), g.Text("9.99")),
	)
	page := benchTable()

	b.Run("string", func(b *testing.B) {
		benchmarkValue(b, "Hello, World!")
	})
	b.Run("bytes", func(b *testing.B) {
		benchmarkValue(b, []byte("Hello, World!"))
	})
	b.Run("context_renderer", func(b *testing.B) {
		benchmarkValue(b, contextNode{node: small})
	})
	b.Run("writer_renderer", func(b *testing.B) {
		benchmarkValue(b, small)
	})
	b.Run("page_prebuilt", func(b *testing.B) {
		benchmarkValue(b, page)
	})
	b.Run("page_built", func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		for range b.N {
			if _, err := render.HTML(ctx, benchTable()); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("example_page", func(b *testing.B) {
		benchmarkValue(b, app.Page(42, app.Live.Href()))
	})
}

func benchmarkValue(b *testing.B, value any) {
	b.Helper()
	ctx := context.Background()
	b.ReportAllocs()
	for range b.N {
		if _, err := render.HTML(ctx, value); err != nil {
			b.Fatal(err)
		}
	}
}

// benchTable builds the products table from Stario's in-process page case.
func benchTable() g.Node {
	rows := make([]g.Node, benchRows)
	for i := range rows {
		rows[i] = h.Tr(
			h.Td(g.Text(strconv.Itoa(i))),
			h.Td(g.Text(fmt.Sprintf("Product %d", i))),
			h.Td(g.Text(fmt.Sprintf("Description of product %d with some text", i))),
			h.Td(g.Text(fmt.Sprintf("%.2f", float64(i)*3.5))),
		)
	}
	return h.HTML(
		h.Head(h.TitleEl(g.Text("Products"))),
		h.Body(
			h.H1(g.Text("Products")),
			h.Table(
				h.THead(h.Tr(
					h.Th(g.Text("Id")),
					h.Th(g.Text("Name")),
					h.Th(g.Text("Description")),
					h.Th(g.Text("Price")),
				)),
				h.TBody(rows...),
			),
		),
	)
}
