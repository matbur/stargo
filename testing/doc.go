// Package stargotest provides helpers for testing Stargo applications.
//
// A Client serves any http.Handler, including a *stargo.App, on a loopback
// test server. It keeps a cookie jar, follows redirects, and reports failures
// through the test interface passed to NewClient:
//
//	import (
//		"net/http"
//		"testing"
//
//		"github.com/matbur/stargo"
//		stargotest "github.com/matbur/stargo/testing"
//	)
//
//	func TestLobby(t *testing.T) {
//		app := stargo.New(stargo.WithTracer(nil))
//		app.HandleFunc("/", func(c *stargo.Ctx) error {
//			return c.HTML("<h1>Rooms</h1>")
//		})
//
//		client := stargotest.NewClient(t, app)
//		client.Get("/").AssertStatus(http.StatusOK).AssertContains("Rooms")
//	}
//
// The package has no DOM or CSS selector engine. Assertions match body
// fragments, and Render renders a page value with the same renderer that
// responses and patches use.
//
// Every helper that can fail calls Fatalf on the client's TB. A failed helper
// stops the test goroutine, so a chained assertion never runs on an invalid
// value.
package stargotest
