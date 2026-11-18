package datastar_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

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
