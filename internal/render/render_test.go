package render_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	g "maragu.dev/gomponents"

	"github.com/matbur/stargo/examples/hello/app"
	"github.com/matbur/stargo/internal/render"
)

type contextRenderer struct {
	calls int
	ctx   context.Context
	body  string
	err   error
}

func (r *contextRenderer) Render(ctx context.Context, w io.Writer) error {
	r.calls++
	r.ctx = ctx
	if r.err != nil {
		return r.err
	}
	_, err := io.WriteString(w, r.body)
	return err
}

type writerRenderer struct {
	calls int
	body  string
}

func (r *writerRenderer) Render(w io.Writer) error {
	r.calls++
	_, err := io.WriteString(w, r.body)
	return err
}

func TestAcceptedForms(t *testing.T) {
	t.Parallel()

	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "value")

	withContext := &contextRenderer{body: "<b>ctx</b>"}
	withWriter := &writerRenderer{body: "<i>writer</i>"}

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"context renderer", withContext, "<b>ctx</b>"},
		{"writer renderer", withWriter, "<i>writer</i>"},
		{"string", "<u>raw</u>", "<u>raw</u>"},
		{"bytes", []byte("<em>raw</em>"), "<em>raw</em>"},
		{"gomponents node", app.Page(0, "/live"), ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := render.HTML(ctx, test.value)
			if err != nil {
				t.Fatalf("HTML() error = %v", err)
			}
			if test.want != "" && string(got) != test.want {
				t.Fatalf("HTML() = %q, want %q", got, test.want)
			}
		})
	}

	if withContext.calls != 1 {
		t.Errorf("context renderer called %d times, want 1", withContext.calls)
	}
	if withContext.ctx != ctx {
		t.Error("context renderer did not receive the request context")
	}
	if withWriter.calls != 1 {
		t.Errorf("writer renderer called %d times, want 1", withWriter.calls)
	}
}

func TestGomponentsBytesMatchDirectRender(t *testing.T) {
	t.Parallel()

	page := app.Page(0, "/live")
	want, err := render.HTML(context.Background(), g.Node(page))
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}

	var direct bytes.Buffer
	if err := page.Render(&direct); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.Equal(want, direct.Bytes()) {
		t.Fatalf("HTML() = %q, direct = %q", want, direct.Bytes())
	}
}

func TestUnsupportedAndNil(t *testing.T) {
	t.Parallel()

	if _, err := render.HTML(context.Background(), 42); err == nil {
		t.Fatal("expected an error for an unsupported value")
	}
	if _, err := render.HTML(context.Background(), nil); err == nil {
		t.Fatal("expected an error for a nil value")
	}
}

func TestRenderErrorReturnsNoBytes(t *testing.T) {
	t.Parallel()

	renderer := &contextRenderer{body: "partial", err: errors.New("boom")}
	body, err := render.HTML(context.Background(), renderer)
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(body) != 0 {
		t.Fatalf("got %d bytes, want none", len(body))
	}
}

func TestTrustedHTMLIsNotEscaped(t *testing.T) {
	t.Parallel()

	body, err := render.HTML(context.Background(), "<b>&</b>")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "<b>&</b>" {
		t.Fatalf("HTML() = %q", body)
	}
	if strings.Contains(string(body), "&amp;") {
		t.Fatal("output was escaped")
	}
}
