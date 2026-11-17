package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo"
)

func serve(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	app := newApp(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })

	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestPage(t *testing.T) {
	t.Parallel()

	recorder := serve(t, http.MethodGet, "/")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}

	body := recorder.Body.String()
	checks := []string{
		"<!doctype html>",
		"<html lang=\"en\">",
		"<title>Hello, Stario</title>",
		"<h1>Hello, Stario</h1>",
		"This page is one handler and one route.",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestPageHasNoClientBehavior(t *testing.T) {
	t.Parallel()

	body := serve(t, http.MethodGet, "/").Body.String()
	for _, unwanted := range []string{"<script", "data-init", "data-signals", "datastar"} {
		if strings.Contains(strings.ToLower(body), unwanted) {
			t.Fatalf("body contains %q: %s", unwanted, body)
		}
	}
}

func TestLiveEndpointIsMissing(t *testing.T) {
	t.Parallel()

	if code := serve(t, http.MethodGet, "/live").Code; code != http.StatusNotFound {
		t.Fatalf("GET /live status = %d, want 404", code)
	}
}
