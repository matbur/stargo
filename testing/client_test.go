package stargotest_test

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	stargotest "github.com/matbur/stargo/testing"
)

func TestClientServesHandler(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "" {
			t.Error("request has no host")
		}
		if r.URL.Path != "/hello" {
			t.Errorf("path = %q, want /hello", r.URL.Path)
		}
		_, _ = w.Write([]byte("hello"))
	})

	client := stargotest.NewClient(t, handler)
	if request := client.Request(http.MethodGet, "/hello"); !request.URL.IsAbs() {
		t.Fatalf("request URL %q is not absolute", request.URL)
	}
	client.Get("/hello").AssertStatus(http.StatusOK).AssertContains("hello")
}

func TestClientResolvesRelativePaths(t *testing.T) {
	t.Parallel()

	client := stargotest.NewClient(t, http.NotFoundHandler())

	root := client.URL("/")
	want := strings.TrimSuffix(root, "/") + "/rooms"
	if got := client.URL("rooms"); got != want {
		t.Fatalf("URL(rooms) = %q, want %q", got, want)
	}
	if got := client.URL("/rooms"); got != want {
		t.Fatalf("URL(/rooms) = %q, want %q", got, want)
	}

	absolute := "https://example.com/path"
	if got := client.URL(absolute); got != absolute {
		t.Fatalf("URL(%q) = %q, want it unchanged", absolute, got)
	}
}

func TestClientClosesServerAtCleanup(t *testing.T) {
	var address string

	t.Run("client", func(t *testing.T) {
		client := stargotest.NewClient(t, http.NotFoundHandler())
		parsed, err := url.Parse(client.URL("/"))
		if err != nil {
			t.Fatalf("parse client URL: %v", err)
		}
		address = parsed.Host
	})

	connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("server at %s still accepts connections after cleanup", address)
	}
}

func TestDefaultHeaders(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.Header.Get("X-Test"))
	})

	client := stargotest.NewClient(t, handler, stargotest.WithDefaultHeader("X-Test", "default"))
	for range 2 {
		client.Get("/").AssertStatus(http.StatusOK).AssertContains("default")
	}
	client.Get("/", stargotest.WithHeader("X-Test", "override")).AssertContains("override")
}

func TestCookieRoundTrip(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			_, _ = w.Write([]byte("set"))
		case "/read":
			cookie, err := r.Cookie("session")
			if err != nil {
				_, _ = w.Write([]byte("missing"))
				return
			}
			_, _ = w.Write([]byte(cookie.Value))
		}
	})

	client := stargotest.NewClient(t, handler)
	client.Get("/set").AssertStatus(http.StatusOK)

	value, ok := client.Cookie("session")
	if !ok || value != "abc" {
		t.Fatalf("Cookie(session) = %q, %v; want %q, true", value, ok, "abc")
	}
	client.Get("/read").AssertContains("abc")
}

func TestWithoutCookies(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		case "/read":
			if len(r.Header.Values("Cookie")) > 0 {
				_, _ = w.Write([]byte("cookies sent"))
				return
			}
			_, _ = w.Write([]byte("no cookies"))
		}
	})

	client := stargotest.NewClient(t, handler, stargotest.WithoutCookies())
	client.Get("/set").AssertStatus(http.StatusOK)

	if value, ok := client.Cookie("session"); ok {
		t.Fatalf("Cookie(session) = %q, true; want the jar disabled", value)
	}
	client.Get("/read").AssertContains("no cookies")
}

func TestFollowsRedirectsByDefault(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/from" {
			http.Redirect(w, r, "/to", http.StatusSeeOther)
			return
		}
		_, _ = w.Write([]byte("target"))
	})

	client := stargotest.NewClient(t, handler)
	client.Get("/from").AssertStatus(http.StatusOK).AssertContains("target")
}

func TestWithoutRedirects(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/to", http.StatusSeeOther)
	})

	client := stargotest.NewClient(t, handler, stargotest.WithoutRedirects())
	client.Get("/from").
		AssertStatus(http.StatusSeeOther).
		AssertHeader("Location", "/to")
}

func TestRedirectLimitFailsTest(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusSeeOther)
	})

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, handler, stargotest.WithMaxRedirects(3))
		client.Get("/loop")
	})

	if !strings.Contains(message, "stopped after 3 redirects") {
		t.Fatalf("failure = %q, want the redirect limit", message)
	}
	if !strings.Contains(message, "/loop") {
		t.Fatalf("failure = %q, want the request target", message)
	}
}

func TestTransportFailureNamesTarget(t *testing.T) {
	t.Parallel()

	message := captureFailure(t, func(tb stargotest.TB) {
		client := stargotest.NewClient(tb, http.NotFoundHandler())
		client.Close()
		client.Get("/unreachable")
	})

	if !strings.Contains(message, "stargotest:") {
		t.Fatalf("failure = %q, want the helper prefix", message)
	}
	if !strings.Contains(message, "/unreachable") {
		t.Fatalf("failure = %q, want the request target", message)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	client := stargotest.NewClient(t, http.NotFoundHandler())
	client.Close()
	client.Close()
}

func TestDefaultHeaderDoesNotOverrideRequestHeader(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Accept")))
	})

	client := stargotest.NewClient(t, handler, stargotest.WithDefaultHeader("Accept", "text/html"))
	request := client.Request(http.MethodGet, "/", stargotest.WithHeader("Accept", "application/json"))

	if got := request.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q, want application/json", got)
	}
}
