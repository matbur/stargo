package assets_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/matbur/stargo/assets"
)

func testMapFS() fstest.MapFS {
	return fstest.MapFS{
		"app.css":          {Data: []byte("body{color:red}")},
		"app.css.br":       {Data: []byte("brotli-bytes")},
		"app.css.gz":       {Data: []byte("gzip-bytes")},
		"app.js":           {Data: []byte("console.log(1)")},
		"nested/img.png":   {Data: []byte("png-bytes")},
		"secret.env":       {Data: []byte("TOKEN=1")},
		".hidden/x.css":    {Data: []byte("hidden")},
		"nested/.keep.css": {Data: []byte("hidden")},
	}
}

func newHandler(t *testing.T, opts ...assets.Option) *assets.Handler {
	t.Helper()
	handler, err := assets.FS(testMapFS(), opts...)
	if err != nil {
		t.Fatalf("FS() error = %v", err)
	}
	return handler
}

func request(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestURLHelpers(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, assets.Prefix("/assets/"))
	hashed := handler.URL("app.css")
	stable := handler.StableURL("app.css")

	if !strings.HasPrefix(hashed, "/assets/app.") || !strings.HasSuffix(hashed, ".css") {
		t.Fatalf("URL = %q", hashed)
	}
	if strings.Count(hashed, ".") != 2 {
		t.Fatalf("URL hash shape = %q", hashed)
	}
	if stable != "/assets/app.css" {
		t.Fatalf("StableURL = %q", stable)
	}
	if handler.URL("app.css") != hashed {
		t.Fatal("URL is not deterministic")
	}
	if handler.URL("nested/img.png") == handler.URL("app.css") {
		t.Fatal("different files share a hashed URL")
	}
}

func TestURLPanicsOnUnknownName(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	defer func() {
		if recover() == nil {
			t.Fatal("URL did not panic for an unknown asset")
		}
	}()
	handler.URL("missing.css")
}

func TestServeHashedAndStable(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, assets.Prefix("/assets"))

	hashed := request(handler, http.MethodGet, handler.URL("app.css"), nil)
	if hashed.Code != http.StatusOK {
		t.Fatalf("hashed status = %d", hashed.Code)
	}
	if hashed.Body.String() != "body{color:red}" {
		t.Fatalf("hashed body = %q", hashed.Body.String())
	}
	if got := hashed.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := hashed.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("hashed Cache-Control = %q", got)
	}
	if hashed.Header().Get("ETag") == "" {
		t.Fatal("hashed response has no ETag")
	}

	stable := request(handler, http.MethodGet, handler.StableURL("app.css"), nil)
	if stable.Code != http.StatusOK {
		t.Fatalf("stable status = %d", stable.Code)
	}
	if stable.Body.String() != "body{color:red}" {
		t.Fatalf("stable body = %q", stable.Body.String())
	}
	if got := stable.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("stable Cache-Control = %q", got)
	}
}

func TestConditionalAndRange(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	target := handler.URL("app.css")

	first := request(handler, http.MethodGet, target, nil)
	etag := first.Header().Get("ETag")

	conditional := request(handler, http.MethodGet, target, map[string]string{"If-None-Match": etag})
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d", conditional.Code)
	}
	if conditional.Body.Len() != 0 {
		t.Fatalf("conditional body = %q", conditional.Body.String())
	}

	ranged := request(handler, http.MethodGet, target, map[string]string{"Range": "bytes=0-3"})
	if ranged.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d", ranged.Code)
	}
	if ranged.Body.String() != "body" {
		t.Fatalf("range body = %q", ranged.Body.String())
	}

	head := request(handler, http.MethodHead, target, nil)
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d", head.Code)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q", head.Body.String())
	}
	if head.Header().Get("Content-Length") == "" {
		t.Fatal("HEAD has no Content-Length")
	}
}

func TestPrecompressedVariants(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	target := handler.StableURL("app.css")

	cases := []struct {
		header   string
		encoding string
		body     string
	}{
		{"br, gzip", "br", "brotli-bytes"},
		{"gzip;q=0.9, br;q=0.2", "gzip", "gzip-bytes"},
		{"gzip", "gzip", "gzip-bytes"},
		{"identity", "", "body{color:red}"},
		{"zstd", "", "body{color:red}"},
	}
	for _, test := range cases {
		recorder := request(handler, http.MethodGet, target, map[string]string{
			"Accept-Encoding": test.header,
		})
		if got := recorder.Header().Get("Content-Encoding"); got != test.encoding {
			t.Errorf("%q: Content-Encoding = %q, want %q", test.header, got, test.encoding)
		}
		if recorder.Body.String() != test.body {
			t.Errorf("%q: body = %q, want %q", test.header, recorder.Body.String(), test.body)
		}
		if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
			t.Errorf("%q: Vary = %q", test.header, got)
		}
	}
}

func TestMissingAssetsAndSafety(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, assets.Prefix("/assets"))
	targets := []string{
		"/assets/secret.env",
		"/assets/.hidden/x.css",
		"/assets/nested/.keep.css",
		"/assets/../secret.env",
		"/assets/nested/",
		"/assets/missing.css",
	}
	for _, target := range targets {
		recorder := request(handler, http.MethodGet, target, nil)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%q: status = %d, want 404", target, recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "TOKEN") {
			t.Errorf("%q: leaked a hidden file", target)
		}
	}

	wrongHash := strings.Replace(handler.URL("app.css"), "app.", "app.0000000000.", 1)
	if recorder := request(handler, http.MethodGet, wrongHash, nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("wrong hash status = %d, want 404", recorder.Code)
	}

	post := request(handler, http.MethodPost, handler.URL("app.css"), nil)
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", post.Code)
	}
	if got := post.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q", got)
	}
}

func TestDirSkipsSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.css"), []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.css")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(root, "escape.css")); err != nil {
		t.Fatal(err)
	}

	handler, err := assets.Dir(root)
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}

	inside := request(handler, http.MethodGet, "/inside.css", nil)
	if inside.Code != http.StatusOK || inside.Body.String() != "inside" {
		t.Fatalf("inside = %d %q", inside.Code, inside.Body.String())
	}
	escape := request(handler, http.MethodGet, "/escape.css", nil)
	if escape.Code != http.StatusNotFound {
		t.Fatalf("symlink status = %d, want 404", escape.Code)
	}
	if strings.Contains(escape.Body.String(), "outside") {
		t.Fatal("symlink target was served")
	}
}

func TestDirRejectsMissingRoot(t *testing.T) {
	t.Parallel()

	if _, err := assets.Dir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Dir() accepted a missing root")
	}
}

func TestAssetNameWithTraversalIsUnknown(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	defer func() {
		if recover() == nil {
			t.Fatal("URL did not panic for a traversal name")
		}
	}()
	handler.URL("../app.css")
}
