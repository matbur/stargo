package main

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/matbur/stargo/auth"
)

func page(title string, body ...g.Node) g.Node {
	return h.Doctype(h.HTML(
		g.Attr("lang", "en"),
		h.Head(
			h.Meta(h.Charset("UTF-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.TitleEl(g.Text(title)),
			h.StyleEl(g.Raw(`
				body {
					font-family: system-ui, sans-serif;
					padding: 2rem;
					max-width: 28rem;
					margin: 0 auto;
				}
				form { display: grid; gap: 0.75rem; margin-block: 1.5rem; }
				label { display: grid; gap: 0.25rem; }
				input, button { font: inherit; padding: 0.5rem; }
				.error { color: #b00020; }
			`)),
		),
		h.Body(body...),
	))
}

func loginPage(message string) g.Node {
	children := []g.Node{
		h.H1(g.Text("Sign in")),
		h.P(g.Text("Demo account: demo / the AUTH_PASSWORD value.")),
	}
	if message != "" {
		children = append(children, h.P(g.Attr("class", "error"), g.Text(message)))
	}
	children = append(children,
		h.Form(
			h.Method("post"),
			h.Action("/login"),
			h.Label(
				g.Text("Username"),
				h.Input(h.Name("username"), h.Value(demoUsername), h.Required()),
			),
			h.Label(
				g.Text("Password"),
				h.Input(h.Type("password"), h.Name("password"), h.Required()),
			),
			h.Button(h.Type("submit"), g.Text("Log in")),
		),
	)
	return page("Sign in", children...)
}

func homePage(user auth.User) g.Node {
	return page("Signed in",
		h.H1(g.Text("Welcome, "+user.Name)),
		h.P(g.Text("This page is only reachable with a session that carries a user.")),
		h.Form(
			h.Method("post"),
			h.Action("/logout"),
			h.Button(h.Type("submit"), g.Text("Log out")),
		),
	)
}
