// Command hello-world is the smallest complete Stargo application: one route,
// one handler, and one HTML page. It loads no Datastar and no JavaScript.
package main

import (
	"log"
	"net/http"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/route"
)

var home = route.Get("/")

func page() g.Node {
	return h.Doctype(h.HTML(
		g.Attr("lang", "en"),
		h.Head(
			h.Meta(h.Charset("UTF-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.TitleEl(g.Text("Hello, Stario")),
			h.StyleEl(g.Raw(`
				body {
					font-family: system-ui, sans-serif;
					padding: 2rem;
					max-width: 36rem;
					margin: 0 auto;
				}
			`)),
		),
		h.Body(
			h.H1(g.Text("Hello, Stario")),
			h.P(g.Text("This page is one handler and one route.")),
		),
	))
}

func newApp(opts ...stargo.Option) *stargo.App {
	app := stargo.New(opts...)
	app.Handle(home, func(c *stargo.Ctx) error {
		// "/" is a subtree pattern in net/http; keep the example to one exact
		// page instead of serving it for every unknown path.
		if c.R.URL.Path != "/" {
			return stargo.NewHTTPError(http.StatusNotFound, "")
		}
		return c.HTML(page())
	})
	return app
}

func main() {
	app := newApp()
	defer app.Close()

	if err := app.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
