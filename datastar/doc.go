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
// New returns an instance for a Datastar bundle built with a custom attribute
// prefix:
//
//	star := ds.New("data-star-")
//	star.On("click", expr)
//
// The prefix must match the bundle loaded by the page, and the same prefix
// must be used for pages, SSE patches, and raw HTML.
//
// Expressions, payload values, and signal filter patterns are trusted
// JavaScript. Never pass user-controlled text as an expression or payload
// value.
package datastar
