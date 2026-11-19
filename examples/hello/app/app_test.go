package app_test

import (
	"context"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/examples/hello/app"
	"github.com/matbur/stargo/route"
	stargotest "github.com/matbur/stargo/testing"
)

func TestPageRendersAndLinksResolve(t *testing.T) {
	t.Parallel()

	application := app.New(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = application.Close() })

	response := stargotest.NewClient(t, application).
		Get("/").
		AssertStatus(http.StatusOK).
		AssertContains(`id="counter"`, "data-init=")

	body := response.Text()
	registered := []route.Route{app.Home, app.Live, app.Command}
	resolves := func(href string) bool {
		for _, handle := range registered {
			if href == handle.Href() {
				return true
			}
		}
		return false
	}

	hrefs := regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(hrefs) == 0 {
		t.Fatalf("page has no links: %q", body)
	}
	for _, match := range hrefs {
		if !resolves(match[1]) {
			t.Fatalf("href %q does not resolve to a registered route", match[1])
		}
	}
	if !resolves(app.Live.Href()) {
		t.Fatal("live route does not build its own href")
	}
}

func TestLiveStreamReceivesPublishedPatch(t *testing.T) {
	t.Parallel()

	application := app.New(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = application.Close() })

	client := stargotest.NewClient(t, application)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream := client.Open(client.Request(http.MethodGet, app.Live.Href()).WithContext(ctx))
	defer stream.Close()

	initial, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no initial patch")
	}
	if initial.Kind != stargotest.PatchElements ||
		initial.Selector != "#counter" ||
		initial.Elements != `<div id="counter">0</div>` {
		t.Fatalf("initial patch = %+v", initial)
	}

	client.Do(client.Request(
		http.MethodPost,
		app.Command.Href(),
		stargotest.WithBody("application/json", []byte(`{}`)),
	)).AssertStatus(http.StatusNoContent)

	updated, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no counter patch")
	}
	if updated.Kind != stargotest.PatchElements ||
		updated.Selector != "#counter" ||
		updated.Elements != `<div id="counter">1</div>` {
		t.Fatalf("counter patch = %+v", updated)
	}
}
