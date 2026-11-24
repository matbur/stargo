package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/matbur/stargo"
	stargotest "github.com/matbur/stargo/testing"
)

const testPassword = "stargo-demo"

func newTestClient(t *testing.T, opts ...stargotest.Option) *stargotest.Client {
	t.Helper()
	app, err := newApp(testPassword, stargo.WithTracer(nil))
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return stargotest.NewClient(t, app, opts...)
}

func credentials(password string) url.Values {
	return url.Values{
		"username": {demoUsername},
		"password": {password},
	}
}

func TestProtectedPageRedirectsToLogin(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, stargotest.WithoutRedirects())
	client.Get("/").
		AssertStatus(http.StatusSeeOther).
		AssertHeader("Location", "/login")

	client.Get("/login").
		AssertStatus(http.StatusOK).
		AssertContains("Sign in", `name="username"`, `name="password"`)
}

func TestWrongPasswordStaysOnLogin(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)
	client.PostForm("/login", credentials("wrong")).
		AssertStatus(http.StatusOK).
		AssertContains("Invalid username or password.")
	if _, ok := client.Cookie("session"); ok {
		t.Fatal("failed login set a session cookie")
	}
}

func TestLoginReusesSessionAndLogoutClearsIt(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)
	client.PostForm("/login", credentials(testPassword)).
		AssertStatus(http.StatusOK).
		AssertContains("Welcome, demo", "Log out")

	cookie, ok := client.Cookie("session")
	if !ok || cookie == "" {
		t.Fatal("login did not set the session cookie")
	}

	client.Get("/").
		AssertStatus(http.StatusOK).
		AssertContains("Welcome, demo")

	client.Get("/missing").AssertStatus(http.StatusNotFound)

	client.PostForm("/logout", nil).
		AssertStatus(http.StatusOK).
		AssertContains("Sign in")
	if _, ok := client.Cookie("session"); ok {
		t.Fatal("logout kept the session cookie")
	}
	client.Get("/").
		AssertStatus(http.StatusOK).
		AssertContains("Sign in")
}

func TestLoginPageRedirectsWhenSignedIn(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)
	client.PostForm("/login", credentials(testPassword)).
		AssertStatus(http.StatusOK).
		AssertContains("Welcome, demo")

	response := client.Get("/login").AssertStatus(http.StatusOK)
	if !strings.Contains(response.Text(), "Welcome, demo") {
		t.Fatalf("signed-in login page = %q", response.Text())
	}
}
