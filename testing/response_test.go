package stargotest_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/matbur/stargo/internal/render"
	stargotest "github.com/matbur/stargo/testing"
)

type writerNode struct {
	html string
}

func (n writerNode) Render(w io.Writer) error {
	_, err := io.WriteString(w, n.html)
	return err
}

func TestResponseTextAndJSON(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Ada","count":2}`))
	})

	client := stargotest.NewClient(t, handler)
	response := client.Get("/")

	if got := response.Text(); got != `{"name":"Ada","count":2}` {
		t.Fatalf("Text() = %q", got)
	}

	var payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	response.DecodeJSON(&payload)
	if payload.Name != "Ada" || payload.Count != 2 {
		t.Fatalf("DecodeJSON() = %+v", payload)
	}
}

func TestAssertionsPassAndChain(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<p>item</p><p>item</p>"))
	})

	client := stargotest.NewClient(t, handler)
	response := client.Get("/")
	if got := response.
		AssertStatus(http.StatusOK).
		AssertHeader("Content-Type", "text/html").
		AssertContains("<p>item</p>", "item").
		AssertNotContains("missing").
		AssertCount("<p>item</p>", 2); got != response {
		t.Fatal("assertions did not return the response")
	}
}

func TestAssertStatusFailure(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
		client.Get("/teapot").AssertStatus(http.StatusOK)
	})

	for _, want := range []string{"GET", "/teapot", "status = 418", "want 200"} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure = %q, want %q", message, want)
		}
	}
}

func TestAssertHeaderFailure(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.NotFoundHandler())
		client.Get("/").AssertHeader("X-Missing", "value")
	})

	if !strings.Contains(message, `header "X-Missing" is missing`) {
		t.Fatalf("failure = %q, want the missing header", message)
	}
	if !strings.Contains(message, `want "value"`) {
		t.Fatalf("failure = %q, want the expected value", message)
	}
}

func TestAssertHeaderValueFailure(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "actual")
		}))
		client.Get("/").AssertHeader("X-Test", "expected")
	})

	for _, want := range []string{`"X-Test"`, `"actual"`, `"expected"`} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure = %q, want %q", message, want)
		}
	}
}

func TestAssertContainsFailureShowsFragmentAndBody(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("actual body"))
		}))
		client.Get("/").AssertContains("missing fragment")
	})

	for _, want := range []string{`"missing fragment"`, "actual body"} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure = %q, want %q", message, want)
		}
	}
}

func TestAssertNotContainsFailureShowsFragment(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("forbidden"))
		}))
		client.Get("/").AssertNotContains("forbidden")
	})

	if !strings.Contains(message, `body contains "forbidden"`) {
		t.Fatalf("failure = %q, want the fragment", message)
	}
}

func TestAssertCountFailure(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("a a a"))
		}))
		client.Get("/").AssertCount("a", 2)
	})

	if !strings.Contains(message, `"a" 3 times, want 2`) {
		t.Fatalf("failure = %q, want the counts", message)
	}
}

func TestDecodeJSONFailureShowsBody(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		var payload map[string]any
		client.Get("/").DecodeJSON(&payload)
	})

	for _, want := range []string{"decode JSON", "not json"} {
		if !strings.Contains(message, want) {
			t.Fatalf("failure = %q, want %q", message, want)
		}
	}
}

func TestFailureTruncatesLargeBody(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 9000)))
		}))
		client.Get("/").AssertContains("missing")
	})

	if !strings.Contains(message, "bytes truncated") {
		t.Fatalf("failure does not mark a truncated body (len %d)", len(message))
	}
}

func TestRenderValues(t *testing.T) {
	t.Parallel()

	if got := stargotest.Render(t, writerNode{html: "<p>node</p>"}); got != "<p>node</p>" {
		t.Fatalf("Render(writer) = %q", got)
	}
	if got := stargotest.Render(t, "<p>string</p>"); got != "<p>string</p>" {
		t.Fatalf("Render(string) = %q", got)
	}
	if got := stargotest.Render(t, []byte("<p>bytes</p>")); got != "<p>bytes</p>" {
		t.Fatalf("Render(bytes) = %q", got)
	}
}

func TestRenderMatchesResponseRenderer(t *testing.T) {
	t.Parallel()

	value := writerNode{html: "<section>same</section>"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := render.HTML(r.Context(), value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	})

	client := stargotest.NewClient(t, handler)
	response := client.Get("/")
	if got, want := stargotest.Render(t, value), response.Text(); got != want {
		t.Fatalf("Render() = %q, response = %q", got, want)
	}
}
