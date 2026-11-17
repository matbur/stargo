package stargo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/matbur/stargo"
	helloapp "github.com/matbur/stargo/examples/hello/app"
	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/sse"
	"github.com/matbur/stargo/trace"
)

const (
	benchRows     = 50
	benchMessage  = "Hello, World!"
	benchJSONBody = `{"name":"Ada","age":42}`
)

// BenchmarkServeHTTP measures the in-process request path: ServeMux dispatch,
// context setup, handler, and response writing. Requests and responses are
// reused, so the numbers exclude socket I/O and HTTP parsing, matching
// Stario's in-process suite.
func BenchmarkServeHTTP(b *testing.B) {
	b.Run("plaintext", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/plaintext", http.StatusOK, func(app *stargo.App) {
			app.Handle(route.Get("/plaintext"), func(c *stargo.Ctx) error {
				return c.Text(benchMessage)
			})
		})
	})

	b.Run("json", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/json", http.StatusOK, func(app *stargo.App) {
			app.Handle(route.Get("/json"), func(c *stargo.Ctx) error {
				return c.JSON(map[string]string{"message": benchMessage})
			})
		})
	})

	b.Run("path_param", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/user/42", http.StatusOK, func(app *stargo.App) {
			app.Handle(route.Get("/user/{id}"), func(c *stargo.Ctx) error {
				id := c.R.PathValue("id")
				return c.JSON(map[string]string{"id": id, "name": "User " + id})
			})
		})
	})

	b.Run("not_found", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/missing", http.StatusNotFound, func(app *stargo.App) {
			app.Handle(route.Get("/plaintext"), func(c *stargo.Ctx) error {
				return c.Text(benchMessage)
			})
		})
	})

	b.Run("method_not_allowed", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodPost, "/plaintext", http.StatusMethodNotAllowed, func(app *stargo.App) {
			app.Handle(route.Get("/plaintext"), func(c *stargo.Ctx) error {
				return c.Text(benchMessage)
			})
		})
	})

	b.Run("page", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/page", http.StatusOK, func(app *stargo.App) {
			app.Handle(route.Get("/page"), func(c *stargo.Ctx) error {
				return c.HTML(benchPage())
			})
		})
	})

	b.Run("traced_plaintext", func(b *testing.B) {
		benchmarkRequest(b, true, http.MethodGet, "/plaintext", http.StatusOK, func(app *stargo.App) {
			app.Handle(route.Get("/plaintext"), func(c *stargo.Ctx) error {
				return c.Text(benchMessage)
			})
		})
	})

	b.Run("json_post", func(b *testing.B) {
		payload := []byte(benchJSONBody)
		application := benchApp(b, false, func(app *stargo.App) {
			app.Handle(route.Post("/validate"), func(c *stargo.Ctx) error {
				var input struct {
					Name string `json:"name"`
					Age  int    `json:"age"`
				}
				if err := json.NewDecoder(c.R.Body).Decode(&input); err != nil {
					return err
				}
				return c.JSON(map[string]any{
					"name":  input.Name,
					"age":   input.Age,
					"valid": true,
				})
			})
		})

		request := httptest.NewRequest(http.MethodPost, "/validate", nil)
		request.Header.Set("Content-Type", "application/json")
		writer := newDiscardWriter()
		writer.reset()
		application.ServeHTTP(writer, withBody(request, payload))
		if writer.status != http.StatusOK {
			b.Fatalf("status = %d, want 200", writer.status)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			request.Body = io.NopCloser(bytes.NewReader(payload))
			request.ContentLength = int64(len(payload))
			writer.reset()
			application.ServeHTTP(writer, request)
		}
	})

	b.Run("stream", func(b *testing.B) {
		benchmarkRequest(b, false, http.MethodGet, "/stream", http.StatusOK, func(app *stargo.App) {
			element := benchElement()
			app.Handle(route.Get("/stream"), func(c *stargo.Ctx) error {
				stream, err := c.Stream()
				if err != nil {
					return err
				}
				for range 4 {
					if err := stream.PatchElements(element); err != nil {
						return nil
					}
					if err := stream.PatchSignals(map[string]any{"count": 1, "name": "x"}); err != nil {
						return nil
					}
				}
				return nil
			})
		})
	})

	b.Run("example_page", func(b *testing.B) {
		application := helloapp.New(stargo.WithTracer(nil))
		b.Cleanup(func() { _ = application.Close() })
		benchmarkApp(b, application, http.MethodGet, "/", http.StatusOK)
	})
}

// BenchmarkSSE measures the per-event encoding cost on one live stream.
func BenchmarkSSE(b *testing.B) {
	element := benchElement()
	b.Run("patch_elements", func(b *testing.B) {
		benchmarkSSEPatch(b, func(stream *sse.Stream) error {
			return stream.PatchElements(element)
		})
	})
	b.Run("patch_signals", func(b *testing.B) {
		benchmarkSSEPatch(b, func(stream *sse.Stream) error {
			return stream.PatchSignals(map[string]any{"count": 1, "name": "x"})
		})
	})
}

func benchmarkRequest(b *testing.B, traced bool, method, target string, wantStatus int, register func(*stargo.App)) {
	b.Helper()
	application := benchApp(b, traced, register)
	benchmarkApp(b, application, method, target, wantStatus)
}

func benchmarkApp(b *testing.B, application *stargo.App, method, target string, wantStatus int) {
	b.Helper()
	request := httptest.NewRequest(method, target, nil)
	writer := newDiscardWriter()

	writer.reset()
	application.ServeHTTP(writer, request)
	if writer.status != wantStatus {
		b.Fatalf("%s %s status = %d, want %d", method, target, writer.status, wantStatus)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		writer.reset()
		application.ServeHTTP(writer, request)
	}
}

func benchmarkSSEPatch(b *testing.B, patch func(*sse.Stream) error) {
	b.Helper()
	writer := newDiscardWriter()
	stream := sse.New(writer)
	if err := patch(stream); err != nil {
		b.Fatalf("first patch: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := patch(stream); err != nil {
			b.Fatalf("patch: %v", err)
		}
	}
}

func benchApp(b *testing.B, traced bool, register func(*stargo.App)) *stargo.App {
	b.Helper()
	options := []stargo.Option{stargo.WithTracer(nil)}
	if traced {
		options = []stargo.Option{
			stargo.WithTracer(trace.New(trace.Destination(io.Discard))),
		}
	}
	application := stargo.New(options...)
	b.Cleanup(func() { _ = application.Close() })
	register(application)
	return application
}

func withBody(request *http.Request, body []byte) *http.Request {
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	return request
}

// benchPage builds the products table from Stario's in-process page case.
func benchPage() g.Node {
	rows := make([]g.Node, benchRows)
	for i := range rows {
		rows[i] = h.Tr(
			h.Td(g.Text(strconv.Itoa(i))),
			h.Td(g.Text(fmt.Sprintf("Product %d", i))),
			h.Td(g.Text(fmt.Sprintf("Description of product %d with some text", i))),
			h.Td(g.Text(fmt.Sprintf("%.2f", float64(i)*3.5))),
		)
	}
	return h.HTML(
		h.Head(h.TitleEl(g.Text("Products"))),
		h.Body(
			h.H1(g.Text("Products")),
			h.Table(
				h.THead(h.Tr(
					h.Th(g.Text("Id")),
					h.Th(g.Text("Name")),
					h.Th(g.Text("Description")),
					h.Th(g.Text("Price")),
				)),
				h.TBody(rows...),
			),
		),
	)
}

// benchElement is the element used by Stario's SSE benchmark case.
func benchElement() g.Node {
	return h.Div(
		g.Attr("id", "tick"),
		g.Attr("class", "card"),
		h.Span(g.Text("tick")),
		h.B(g.Text("bold")),
	)
}

// discardWriter is an in-process response sink: headers are kept in a map and
// bodies are counted but discarded. FlushError makes SSE writes take the same
// response-controller path as a live connection.
type discardWriter struct {
	header http.Header
	status int
	bytes  int
}

func newDiscardWriter() *discardWriter {
	return &discardWriter{header: make(http.Header)}
}

func (w *discardWriter) Header() http.Header {
	return w.header
}

func (w *discardWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *discardWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.bytes += len(body)
	return len(body), nil
}

func (w *discardWriter) Flush() {}

func (w *discardWriter) FlushError() error { return nil }

func (w *discardWriter) reset() {
	w.status = 0
	w.bytes = 0
	clear(w.header)
}
