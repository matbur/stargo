package datastar_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	ds "github.com/matbur/stargo/datastar"
	"github.com/matbur/stargo/internal/render"
	"github.com/matbur/stargo/sse"
)

func TestComposesWithGomponents(t *testing.T) {
	got := renderNode(t, h.Div(ds.On("click", "@post('/x')"), g.Text("go")))
	want := `<div data-on:click="@post(&#39;/x&#39;)">go</div>`
	if got != want {
		t.Fatalf("composed = %q, want %q", got, want)
	}
}

func TestPageAndPatchParity(t *testing.T) {
	node := h.Div(ds.On("click", ds.Post("/command")), g.Text("go"))

	page, err := render.HTML(context.Background(), node)
	if err != nil {
		t.Fatalf("page render: %v", err)
	}

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements(node, sse.WithSelector("#go")); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, string(page)) {
		t.Fatalf("patch body does not contain page HTML %q:\n%s", page, body)
	}
}

func TestReactiveHelpersCompose(t *testing.T) {
	node := h.Div(
		ds.Init("@get('/live')"),
		ds.Signals(map[string]any{"count": 0}, ds.WithIfMissing()),
		ds.Bind("message"),
		ds.Attr("title", "$title"),
		ds.Show("$open"),
		ds.Text("$label"),
		ds.Effect("el.focus()"),
		ds.Class("hidden", "!$open"),
		ds.Style("width", "$pct + '%'"),
		ds.Computed("total", "$a + $b"),
		g.Text("x"),
	)
	got := renderNode(t, node)
	opening := got[:strings.Index(got, ">")+1]
	for _, want := range []string{
		`data-init="@get(&#39;/live&#39;)"`,
		`data-signals__ifmissing="{&#34;count&#34;:0}"`,
		`data-bind="message"`,
		`data-attr:title="$title"`,
		`data-show="$open"`,
		`data-text="$label"`,
		`data-effect="el.focus()"`,
		`data-class:hidden="!$open"`,
		`data-style:width="$pct + &#39;%&#39;"`,
		`data-computed="{&#39;total&#39;:() =&gt; ($a + $b)}"`,
	} {
		if !strings.Contains(opening, want) {
			t.Fatalf("opening tag %q missing %q", opening, want)
		}
	}
	if !strings.HasSuffix(got, ">x</div>") {
		t.Fatalf("composed = %q", got)
	}
}

func TestReactivePageAndPatchParity(t *testing.T) {
	node := h.Div(
		ds.Bind("message"),
		ds.Signals(map[string]any{"ui.open": true}),
		ds.Computed("total", "$a + $b"),
		g.Text("go"),
	)
	page, err := render.HTML(context.Background(), node)
	if err != nil {
		t.Fatalf("page render: %v", err)
	}

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements(node, sse.WithSelector("#go")); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if body := recorder.Body.String(); !strings.Contains(body, string(page)) {
		t.Fatalf("patch body does not contain page HTML %q:\n%s", page, body)
	}
}

func TestExtendedHelpersCompose(t *testing.T) {
	node := h.Div(
		ds.Attrs(map[string]string{"open": "sidebarOpen"}),
		ds.Ignore(),
		ds.OnSignalPatch("save()",
			ds.WithSignalPatchDebounce(500*time.Millisecond),
			ds.WithSignalPatchInclude("draft"),
		),
		g.Text("x"),
	)
	got := renderNode(t, node)
	opening := got[:strings.Index(got, ">")+1]
	for _, want := range []string{
		`data-attr="{&#39;open&#39;:sidebarOpen}"`,
		`data-ignore`,
		`data-on-signal-patch__debounce.500ms="save()"`,
		`data-on-signal-patch-filter="{&#39;include&#39;:&#39;draft&#39;}"`,
	} {
		if !strings.Contains(opening, want) {
			t.Fatalf("opening tag %q missing %q", opening, want)
		}
	}
	if !strings.HasSuffix(got, ">x</div>") {
		t.Fatalf("composed = %q", got)
	}
}

func TestExtendedPageAndPatchParity(t *testing.T) {
	node := h.Div(
		ds.Ignore(),
		ds.OnSignalPatch("save()",
			ds.WithSignalPatchThrottle(250*time.Millisecond),
			ds.WithSignalPatchExclude("secret"),
		),
		g.Text("go"),
	)
	page, err := render.HTML(context.Background(), node)
	if err != nil {
		t.Fatalf("page render: %v", err)
	}

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder)
	if err := stream.PatchElements(node, sse.WithSelector("#go")); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if body := recorder.Body.String(); !strings.Contains(body, string(page)) {
		t.Fatalf("patch body does not contain page HTML %q:\n%s", page, body)
	}
}
