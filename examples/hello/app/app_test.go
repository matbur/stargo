package app_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/examples/hello/app"
	"github.com/matbur/stargo/route"
)

func TestPageRendersAndLinksResolve(t *testing.T) {
	t.Parallel()

	application := app.New(stargo.WithTracer(nil))
	recorder := httptest.NewRecorder()
	application.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `id="counter"`) {
		t.Fatalf("counter missing from page: %q", body)
	}
	if !strings.Contains(body, "data-init=") {
		t.Fatalf("live stream is not connected: %q", body)
	}

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
	application := app.New(stargo.WithTracer(nil))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	baseURL := "http://" + listener.Addr().String()

	ctx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serveErr := make(chan error, 1)
	go func() { serveErr <- application.Serve(ctx, listener) }()

	requestCtx, cancelRequest := context.WithCancel(ctx)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("GET /live: %v", err)
	}
	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)
	readUntil(t, scanner, `data: elements <div id="counter">0</div>`, 3*time.Second)

	commandResponse, err := client.Post(baseURL+"/command", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /command: %v", err)
	}
	_, _ = io.Copy(io.Discard, commandResponse.Body)
	_ = commandResponse.Body.Close()
	if commandResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("command status = %d", commandResponse.StatusCode)
	}

	readUntil(t, scanner, `data: elements <div id="counter">1</div>`, 3*time.Second)

	cancelRequest()
	cancelServer()
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve() did not return after cancelling")
	}
}

// readUntil scans lines until one contains needle. The scanner is tied to the
// live response body, so cancellation closes it.
func readUntil(t *testing.T, scanner *bufio.Scanner, needle string, timeout time.Duration) {
	t.Helper()
	done := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), needle) {
				done <- true
				return
			}
		}
		done <- false
	}()

	select {
	case found := <-done:
		if !found {
			t.Fatalf("stream ended before %q", needle)
		}
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %q", needle)
	}
}
