package app

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	ds "github.com/matbur/stargo/datastar"
)

// DatastarScript is the Datastar browser bundle used by the example. Pin a
// version instead of a branch in a real deployment.
const DatastarScript = "https://cdn.jsdelivr.net/gh/starfederation/datastar@main/bundles/datastar.js"

// Page is the initial HTML for the counter example.
//
// The body connects the live stream with data-init, and the button sends a
// command. The live href is passed in so the example builds links from its
// route handles.
func Page(count int64, liveHref string) g.Node {
	return h.Doctype(h.HTML(
		h.Head(
			h.TitleEl(g.Text("Stargo counter")),
			h.Script(h.Type("module"), h.Src(DatastarScript)),
		),
		h.Body(
			g.Attr("data-init", ds.Get("/live")),
			h.H1(g.Text("Stargo counter")),
			Counter(count),
			h.Button(
				ds.On("click", ds.Post("/command")),
				g.Text("Increment"),
			),
			h.P(
				h.A(h.Href(liveHref), g.Text("Open the live stream directly")),
			),
		),
	))
}

// Counter is the fragment patched over the live stream.
func Counter(count int64) g.Node {
	return h.Div(g.Attr("id", "counter"), g.Text(fmt.Sprint(count)))
}
