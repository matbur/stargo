package sse_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/matbur/stargo/sse"
)

func streamRequest(acceptEncoding string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/live", nil)
	if acceptEncoding != "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	return request
}

func decodeGzip(t *testing.T, body []byte) string {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(decoded)
}

func decodeBrotli(t *testing.T, body []byte) string {
	t.Helper()
	decoded, err := io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("brotli decode error = %v", err)
	}
	return string(decoded)
}

func TestEncodingNegotiation(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"gzip":                 "gzip",
		"GZIP":                 "gzip",
		"x-gzip":               "gzip",
		"br":                   "br",
		"BR":                   "br",
		"br, gzip":             "br",
		"gzip, br":             "br",
		"br;q=0.2, gzip;q=0.9": "gzip",
		"br;q=0.9, gzip;q=0.2": "br",
		"br;q=0, gzip":         "gzip",
		"gzip;q=0":             "",
		"br;q=0":               "",
		"*":                    "br",
		"*;q=0":                "",
		"*;q=0, gzip":          "gzip",
		"identity":             "",
		"":                     "",
		"gzip;q=bogus":         "",
		"gzip, *;q=0.1":        "gzip",
	}
	for header, want := range cases {
		recorder := httptest.NewRecorder()
		stream := sse.New(recorder, sse.WithRequest(streamRequest(header)))
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			t.Fatalf("%q: PatchElements() error = %v", header, err)
		}
		if got := recorder.Header().Get("Content-Encoding"); got != want {
			t.Errorf("%q: Content-Encoding = %q, want %q", header, got, want)
		}
	}
}

func TestGzipStreamHeadersAndBody(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder, sse.WithRequest(streamRequest("gzip")))
	if err := stream.PatchElements("<div>first</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if err := stream.PatchElements("<div>second</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q", got)
	}
	if got := strings.Join(recorder.Header().Values("Vary"), ","); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q", got)
	}

	body := decodeGzip(t, recorder.Body.Bytes())
	if !strings.Contains(body, "data: elements <div>first</div>") {
		t.Fatalf("body missing first event: %q", body)
	}
	if !strings.Contains(body, "data: elements <div>second</div>") {
		t.Fatalf("body missing second event: %q", body)
	}
	if strings.Index(body, "first") > strings.Index(body, "second") {
		t.Fatalf("events are out of order: %q", body)
	}
}

func TestBrotliStreamHeadersAndBody(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder, sse.WithRequest(streamRequest("br")))
	if err := stream.PatchElements("<div>first</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if err := stream.PatchSignals(map[string]any{"count": 2}); err != nil {
		t.Fatalf("PatchSignals() error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := recorder.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q", got)
	}
	body := decodeBrotli(t, recorder.Body.Bytes())
	if !strings.Contains(body, "data: elements <div>first</div>") {
		t.Fatalf("body missing element patch: %q", body)
	}
	if !strings.Contains(body, `data: signals {"count":2}`) {
		t.Fatalf("body missing signal patch: %q", body)
	}
}

func TestUncompressedStreamUnchanged(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder, sse.WithRequest(streamRequest("")))
	if err := stream.PatchElements("<div>x</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := recorder.Header().Get("Vary"); got != "" {
		t.Fatalf("Vary = %q, want empty", got)
	}
	if !strings.Contains(recorder.Body.String(), "data: elements <div>x</div>") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestCompressionSafety(t *testing.T) {
	t.Parallel()

	t.Run("existing encoding", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Content-Encoding", "br")
		stream := sse.New(recorder, sse.WithRequest(streamRequest("gzip")))
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			t.Fatalf("PatchElements() error = %v", err)
		}
		if got := recorder.Header().Get("Content-Encoding"); got != "br" {
			t.Fatalf("Content-Encoding = %q, want br", got)
		}
		if strings.Contains(recorder.Body.String(), "\x1f\x8b") {
			t.Fatal("body was gzip-compressed")
		}
	})

	t.Run("no transform", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Cache-Control", "no-transform")
		stream := sse.New(recorder, sse.WithRequest(streamRequest("gzip")))
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			t.Fatalf("PatchElements() error = %v", err)
		}
		if got := recorder.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-transform, no-cache" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})

	t.Run("head request", func(t *testing.T) {
		request := streamRequest("gzip")
		request.Method = http.MethodHead
		recorder := httptest.NewRecorder()
		stream := sse.New(recorder, sse.WithRequest(request))
		if err := stream.PatchElements("<div>x</div>"); err != nil {
			t.Fatalf("PatchElements() error = %v", err)
		}
		if got := recorder.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
	})
}

type flushRecorder struct {
	header     http.Header
	body       bytes.Buffer
	flushes    int
	flushSizes []int
}

func (r *flushRecorder) Header() http.Header {
	return r.header
}

func (r *flushRecorder) WriteHeader(int) {}

func (r *flushRecorder) Write(p []byte) (int, error) {
	return r.body.Write(p)
}

func (r *flushRecorder) Flush() {
	r.flushes++
	r.flushSizes = append(r.flushSizes, r.body.Len())
}

func TestCompressedStreamFlushesEachEvent(t *testing.T) {
	t.Parallel()

	recorder := &flushRecorder{header: http.Header{}}
	stream := sse.New(recorder, sse.WithRequest(streamRequest("gzip")))
	if err := stream.PatchElements("<div>one</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if err := stream.PatchElements("<div>two</div>"); err != nil {
		t.Fatalf("PatchElements() error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if recorder.flushes != 2 {
		t.Fatalf("flushes = %d, want 2", recorder.flushes)
	}
	if len(recorder.flushSizes) != 2 {
		t.Fatalf("flush sizes = %v", recorder.flushSizes)
	}
	if recorder.flushSizes[0] == 0 || recorder.flushSizes[1] <= recorder.flushSizes[0] {
		t.Fatalf("flush sizes = %v", recorder.flushSizes)
	}
}

func TestCloseWithoutEventsWritesNothing(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	stream := sse.New(recorder, sse.WithRequest(streamRequest("gzip")))
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %d bytes, want 0", recorder.Body.Len())
	}
}
