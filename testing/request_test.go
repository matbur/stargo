package stargotest_test

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matbur/stargo/forms"
	stargotest "github.com/matbur/stargo/testing"
)

func TestRequestEchoesMethodHeadersAndBody(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = fmt.Fprintf(
			w,
			"method=%s content-type=%s x-test=%s body=%s",
			r.Method, r.Header.Get("Content-Type"), r.Header.Get("X-Test"), body,
		)
	})

	client := stargotest.NewClient(t, handler)
	response := client.Do(client.Request(
		http.MethodPost,
		"/echo",
		stargotest.WithHeader("X-Test", "value"),
		stargotest.WithBody("text/plain", []byte("payload")),
	))

	response.AssertStatus(http.StatusOK).AssertContains(
		"method=POST",
		"content-type=text/plain",
		"x-test=value",
		"body=payload",
	)
}

func TestGetUsesQueryAndHeaders(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s|%s", r.URL.Query().Get("q"), r.Header.Get("X-Test"))
	})

	client := stargotest.NewClient(t, handler)
	client.Get("/search?q=rooms", stargotest.WithHeader("X-Test", "yes")).
		AssertStatus(http.StatusOK).
		AssertContains("rooms|yes")
}

func TestWithJSON(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = fmt.Fprintf(w, "%s|%s", r.Header.Get("Content-Type"), body)
	})

	client := stargotest.NewClient(t, handler)
	client.Do(client.Request(http.MethodPost, "/", stargotest.WithJSON(map[string]int{"count": 2}))).
		AssertContains(`application/json|{"count":2}`)
}

func TestPostFormPreservesValues(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form, err := forms.Read(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer form.Close()
		_, _ = fmt.Fprintf(
			w,
			"tags=%s content-type=%s",
			strings.Join(form.Values("tag"), ","),
			r.Header.Get("Content-Type"),
		)
	})

	client := stargotest.NewClient(t, handler)
	client.PostForm("/", url.Values{"tag": {"a", "b"}}).
		AssertStatus(http.StatusOK).
		AssertContains("tags=a,b", "content-type=application/x-www-form-urlencoded")
}

func TestPostMultipartFieldsAndFiles(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form, err := forms.Read(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer form.Close()

		_, _ = fmt.Fprintf(w, "title=%s tags=%s", form.Get("title"), strings.Join(form.Values("tag"), ","))
		for _, upload := range form.Files("upload") {
			content, readErr := upload.ReadAll()
			if readErr != nil {
				http.Error(w, readErr.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = fmt.Fprintf(w, "|%s:%s:%s", upload.Filename, upload.ContentType, content)
		}
	})

	client := stargotest.NewClient(t, handler)
	response := client.PostMultipart("/", stargotest.Multipart{
		Fields: url.Values{
			"title": {"Report"},
			"tag":   {"a", "b"},
		},
		Files: []stargotest.File{
			{Field: "upload", Name: "first.txt", ContentType: "text/plain", Content: []byte("one")},
			{Field: "upload", Name: "second.bin", Content: []byte("two")},
		},
	})

	response.AssertStatus(http.StatusOK).AssertContains(
		"title=Report",
		"tags=a,b",
		"|first.txt:text/plain:one",
		"|second.bin:application/octet-stream:two",
	)
}

func TestMultipartBoundaryMatchesBody(t *testing.T) {
	t.Parallel()

	client := stargotest.NewClient(t, http.NotFoundHandler())
	request := client.Request(http.MethodPost, "/", stargotest.WithMultipart(stargotest.Multipart{
		Fields: url.Values{"name": {"Ada"}},
	}))

	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("media type = %q, want multipart/form-data", mediaType)
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("content type has no boundary")
	}
	if !strings.Contains(string(body), "--"+boundary) {
		t.Fatalf("body does not use boundary %q", boundary)
	}
}

func TestMultipartFieldOrderIsDeterministic(t *testing.T) {
	t.Parallel()

	client := stargotest.NewClient(t, http.NotFoundHandler())
	form := stargotest.Multipart{Fields: url.Values{"b": {"2"}, "a": {"1"}}}

	first := client.Request(http.MethodPost, "/", stargotest.WithMultipart(form))
	second := client.Request(http.MethodPost, "/", stargotest.WithMultipart(form))

	firstBody, _ := io.ReadAll(first.Body)
	secondBody, _ := io.ReadAll(second.Body)
	firstBoundary := first.Header.Get("Content-Type")
	secondBoundary := second.Header.Get("Content-Type")
	if firstBoundary == secondBoundary {
		t.Fatal("two multipart requests reused the same boundary")
	}
	firstText := strings.ReplaceAll(string(firstBody), strings.Split(firstBoundary, "boundary=")[1], "BOUNDARY")
	secondText := strings.ReplaceAll(string(secondBody), strings.Split(secondBoundary, "boundary=")[1], "BOUNDARY")
	if firstText != secondText {
		t.Fatalf("multipart bodies differ:\n%s\n%s", firstText, secondText)
	}
}

func TestRequestReachesTestServer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("external"))
	}))
	defer server.Close()

	client := stargotest.NewClient(t, http.NotFoundHandler())
	client.Get(server.URL).AssertContains("external")
}
