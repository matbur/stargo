package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/matbur/stargo"
	stargotest "github.com/matbur/stargo/testing"
)

func serve(t *testing.T, method, target string) *stargotest.Response {
	t.Helper()

	app := newApp(stargo.WithTracer(nil))
	t.Cleanup(func() { _ = app.Close() })

	client := stargotest.NewClient(t, app)
	return client.Do(client.Request(method, target))
}

func TestPage(t *testing.T) {
	t.Parallel()

	serve(t, http.MethodGet, "/").
		AssertStatus(http.StatusOK).
		AssertHeader("Content-Type", "text/html; charset=utf-8").
		AssertContains(
			"<!doctype html>",
			`<html lang="en">`,
			"<title>Hello, Stario</title>",
			"<h1>Hello, Stario</h1>",
			"This page is one handler and one route.",
		)
}

func TestPageHasNoClientBehavior(t *testing.T) {
	t.Parallel()

	body := strings.ToLower(serve(t, http.MethodGet, "/").Text())
	for _, unwanted := range []string{"<script", "data-init", "data-signals", "datastar"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("body contains %q: %s", unwanted, body)
		}
	}
}

func TestLiveEndpointIsMissing(t *testing.T) {
	t.Parallel()

	serve(t, http.MethodGet, "/live").AssertStatus(http.StatusNotFound)
}
