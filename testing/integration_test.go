package stargotest_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/cookies"
	"github.com/matbur/stargo/forms"
	"github.com/matbur/stargo/sse"
	stargotest "github.com/matbur/stargo/testing"
)

func TestClientAgainstStargoApp(t *testing.T) {
	t.Parallel()

	app := stargo.New(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })

	app.HandleFunc("/set-cookie", func(c *stargo.Ctx) error {
		if err := cookies.Set(c.W, &http.Cookie{Name: "visit", Value: "1", Path: "/"}); err != nil {
			return err
		}
		return c.Text("set")
	})

	app.HandleFunc("/submit", func(c *stargo.Ctx) error {
		form, err := forms.Read(c.R)
		if err != nil {
			return stargo.NewHTTPError(http.StatusBadRequest, "invalid form")
		}
		defer form.Close()

		if err := cookies.Set(c.W, &http.Cookie{Name: "echo", Value: "yes", Path: "/"}); err != nil {
			return err
		}
		return c.Text(fmt.Sprintf(
			"name=%s tags=%s visit=%s",
			form.Get("name"),
			strings.Join(form.Values("tag"), ","),
			cookies.Read(c.R)["visit"],
		))
	})

	app.HandleFunc("/events", func(c *stargo.Ctx) error {
		stream, err := c.Stream()
		if err != nil {
			return err
		}
		if err := stream.Ping("keepalive"); err != nil {
			return err
		}
		if err := stream.PatchElements(
			`<div id="feed">Hello</div>`,
			sse.WithMode(sse.ModeAppend),
			sse.WithSelector("#feed"),
		); err != nil {
			return err
		}
		return stream.PatchSignals(
			map[string]any{"count": 1},
			sse.WithOnlyIfMissing(),
		)
	})

	renderValue := writerNode{html: "<p>rendered</p>"}
	app.HandleFunc("/render", func(c *stargo.Ctx) error {
		return c.HTML(renderValue)
	})

	client := stargotest.NewClient(t, app)

	client.Get("/set-cookie").AssertStatus(http.StatusOK)
	client.PostForm("/submit", url.Values{
		"name": {"Ada"},
		"tag":  {"a", "b"},
	}).AssertStatus(http.StatusOK).AssertContains("name=Ada", "tags=a,b", "visit=1")

	if value, ok := client.Cookie("echo"); !ok || value != "yes" {
		t.Fatalf("Cookie(echo) = %q, %v", value, ok)
	}

	stream := client.Open(client.Request(http.MethodGet, "/events"))
	defer stream.Close()

	if stream.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200", stream.StatusCode)
	}
	if got := stream.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}

	element, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no element patch")
	}
	if element.Kind != stargotest.PatchElements ||
		element.Mode != string(sse.ModeAppend) ||
		element.Selector != "#feed" ||
		element.Elements != `<div id="feed">Hello</div>` {
		t.Fatalf("element patch = %+v", element)
	}

	signals, ok := stream.NextPatch()
	if !ok {
		t.Fatal("NextPatch() returned no signal patch")
	}
	if signals.Kind != stargotest.PatchSignals || !signals.OnlyIfMissing {
		t.Fatalf("signal patch = %+v", signals)
	}
	if signals.Signals["count"] != float64(1) {
		t.Fatalf("signals = %+v", signals.Signals)
	}
	if _, ok := stream.NextPatch(); ok {
		t.Fatal("NextPatch() returned a patch after the stream ended")
	}

	client.Get("/render").AssertStatus(http.StatusOK)
	response := client.Get("/render")
	if got, want := stargotest.Render(t, renderValue), response.Text(); got != want {
		t.Fatalf("Render() = %q, response = %q", got, want)
	}
}
