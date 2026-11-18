// Package datastar builds Datastar attributes and action expressions from
// typed inputs instead of raw attribute strings.
//
// Import it as ds:
//
//	import ds "github.com/matbur/stargo/datastar"
//
//	ds.On("click", ds.Post("/cart"))
//	ds.On("submit", submit, ds.WithPrevent())
//	ds.On("input", typing, ds.WithDebounce(500*time.Millisecond))
//
// Actions accept the same options as Datastar's fetch actions:
//
//	ds.Post("/rooms", ds.WithForm(), ds.WithSelector("#create-room"))
//
// State and binding attributes cover the rest of the Datastar surface:
//
//	ds.Init(ds.Get("/live"))
//	ds.Signals(map[string]any{"count": 0}, ds.WithIfMissing())
//	ds.Bind("message")
//	ds.Attr("disabled", "!$message")
//
// New returns an instance for a Datastar bundle built with a custom attribute
// prefix:
//
//	star := ds.New("data-star-")
//	star.On("click", expr)
//
// The prefix must match the bundle loaded by the page, and the same prefix
// must be used for pages, SSE patches, and raw HTML.
//
// Signal payloads are JSON data. Expressions, action payload values, and
// signal filter patterns are trusted JavaScript; never pass user-controlled
// text as an expression or payload value.
package datastar
