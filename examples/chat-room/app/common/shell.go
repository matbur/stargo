package common

import (
	_ "embed"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DatastarScript is the pinned Datastar browser bundle.
const DatastarScript = "https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.4/bundles/datastar.js"

//go:embed style.css
var styleCSS string

// Page wraps feature content in the shared document shell.
func Page(body ...g.Node) g.Node {
	return h.Doctype(h.HTML(
		g.Attr("lang", "en"),
		h.Head(
			h.Meta(h.Charset("UTF-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.TitleEl(g.Text("Stario Chat")),
			h.StyleEl(g.Raw(styleCSS)),
			h.Script(h.Type("module"), h.Src(DatastarScript)),
		),
		h.Body(g.Group(body)),
	))
}
