package cookies_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matbur/stargo/cookies"
)

var (
	newKey = strings.Repeat("n", 32)
	oldKey = strings.Repeat("o", 32)
)

func signedValue(t *testing.T, signer *cookies.Signer, data map[string]any) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	if err := signer.Set(recorder, data); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return cookieValue(t, singleHeader(t, recorder))
}

func TestSignerRoundTrip(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey}, cookies.WithMaxAge(time.Hour))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := signer.Set(recorder, map[string]any{"user": "42"}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	header := singleHeader(t, recorder)
	for _, attribute := range []string{"Path=/", "Max-Age=3600", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(header, attribute) {
			t.Errorf("Set-Cookie = %q, want %q", header, attribute)
		}
	}

	value := cookieValue(t, header)
	got, ok := signer.Get(requestWithCookies("session=" + value))
	if !ok {
		t.Fatal("Get() reported no value")
	}
	if got.Data["user"] != "42" {
		t.Errorf("data = %v, want user 42", got.Data)
	}
	if delta := time.Since(got.IssuedAt); delta < 0 || delta > 5*time.Second {
		t.Errorf("IssuedAt = %v, want within 5s of now", got.IssuedAt)
	}
	if got.ExpiresAt.Sub(got.IssuedAt) != time.Hour {
		t.Errorf("ExpiresAt - IssuedAt = %v, want 1h", got.ExpiresAt.Sub(got.IssuedAt))
	}
}

func TestSignerDefaults(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	header := singleHeader(t, setSignerCookie(t, signer))
	for _, attribute := range []string{"Path=/", "Max-Age=1209600", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(header, attribute) {
			t.Errorf("Set-Cookie = %q, want %q", header, attribute)
		}
	}
}

func setSignerCookie(t *testing.T, signer *cookies.Signer) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	if err := signer.Set(recorder, map[string]any{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return recorder
}

func TestSignerOptions(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey},
		cookies.WithMaxAge(time.Hour),
		cookies.WithPath("/app"),
		cookies.WithDomain("example.com"),
		cookies.WithSecure(false),
		cookies.WithHTTPOnly(false),
		cookies.WithSameSite(http.SameSiteStrictMode),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	header := singleHeader(t, setSignerCookie(t, signer))
	for _, attribute := range []string{"Path=/app", "Domain=example.com", "Max-Age=3600", "SameSite=Strict"} {
		if !strings.Contains(header, attribute) {
			t.Errorf("Set-Cookie = %q, want %q", header, attribute)
		}
	}
	for _, absent := range []string{"Secure", "HttpOnly"} {
		if strings.Contains(header, absent) {
			t.Errorf("Set-Cookie = %q, want no %s", header, absent)
		}
	}
}

func TestSignerValidation(t *testing.T) {
	cases := map[string]func() (*cookies.Signer, error){
		"empty name": func() (*cookies.Signer, error) {
			return cookies.NewSigner("", []string{newKey})
		},
		"no keys": func() (*cookies.Signer, error) {
			return cookies.NewSigner("session", nil)
		},
		"short key": func() (*cookies.Signer, error) {
			return cookies.NewSigner("session", []string{"short"})
		},
		"zero max age": func() (*cookies.Signer, error) {
			return cookies.NewSigner("session", []string{newKey}, cookies.WithMaxAge(0))
		},
		"negative max age": func() (*cookies.Signer, error) {
			return cookies.NewSigner("session", []string{newKey}, cookies.WithMaxAge(-time.Hour))
		},
		"host without secure": func() (*cookies.Signer, error) {
			return cookies.NewSigner("__Host-session", []string{newKey}, cookies.WithSecure(false))
		},
		"host with domain": func() (*cookies.Signer, error) {
			return cookies.NewSigner("__Host-session", []string{newKey}, cookies.WithDomain("example.com"))
		},
		"host wrong path": func() (*cookies.Signer, error) {
			return cookies.NewSigner("__Host-session", []string{newKey}, cookies.WithPath("/app"))
		},
		"secure without secure": func() (*cookies.Signer, error) {
			return cookies.NewSigner("__Secure-session", []string{newKey}, cookies.WithSecure(false))
		},
	}
	for name, newSigner := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := newSigner(); !errors.Is(err, cookies.ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}

	for _, name := range []string{"__Host-session", "__Secure-session"} {
		t.Run("valid "+name, func(t *testing.T) {
			if _, err := cookies.NewSigner(name, []string{newKey}); err != nil {
				t.Fatalf("new signer: %v", err)
			}
		})
	}
}

func TestSignerMissingCookie(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	if _, ok := signer.Get(requestWithCookies()); ok {
		t.Error("Get() reported a value for a request without cookies")
	}
}

func TestSignerTampering(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	value := signedValue(t, signer, map[string]any{"user": "42"})
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		t.Fatalf("value %q has %d parts, want 3", value, len(parts))
	}

	flip := func(text string) string {
		if text[0] == 'A' {
			return "B" + text[1:]
		}
		return "A" + text[1:]
	}

	cases := map[string]string{
		"changed payload":   parts[0] + "." + flip(parts[1]) + "." + parts[2],
		"changed signature": parts[0] + "." + parts[1] + "." + flip(parts[2]),
		"changed version":   "v2." + parts[1] + "." + parts[2],
		"garbage":           "not-a-cookie",
	}
	for name, tampered := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := signer.Get(requestWithCookies("session=" + tampered)); ok {
				t.Error("Get() accepted a changed value")
			}
		})
	}
}

func TestSignerValueCannotMoveToAnotherName(t *testing.T) {
	alpha, err := cookies.NewSigner("alpha", []string{newKey})
	if err != nil {
		t.Fatalf("new alpha signer: %v", err)
	}
	beta, err := cookies.NewSigner("beta", []string{newKey})
	if err != nil {
		t.Fatalf("new beta signer: %v", err)
	}

	value := signedValue(t, alpha, map[string]any{"user": "42"})
	if _, ok := alpha.Get(requestWithCookies("alpha=" + value)); !ok {
		t.Error("alpha did not accept its own value")
	}
	if _, ok := beta.Get(requestWithCookies("beta=" + value)); ok {
		t.Error("beta accepted a value signed for alpha")
	}
}

func TestSignerRotation(t *testing.T) {
	multi, err := cookies.NewSigner("session", []string{newKey, oldKey})
	if err != nil {
		t.Fatalf("new multi signer: %v", err)
	}
	old, err := cookies.NewSigner("session", []string{oldKey})
	if err != nil {
		t.Fatalf("new old signer: %v", err)
	}
	newOnly, err := cookies.NewSigner("session", []string{newKey})
	if err != nil {
		t.Fatalf("new new signer: %v", err)
	}

	oldValue := signedValue(t, old, map[string]any{"user": "old"})
	if _, ok := multi.Get(requestWithCookies("session=" + oldValue)); !ok {
		t.Error("multi-key signer rejected a value signed with the old key")
	}

	newValue := signedValue(t, multi, map[string]any{"user": "new"})
	if _, ok := newOnly.Get(requestWithCookies("session=" + newValue)); !ok {
		t.Error("value was not signed with the first key")
	}
	if _, ok := old.Get(requestWithCookies("session=" + newValue)); ok {
		t.Error("old key accepted a value signed with the new key")
	}
}

func TestSignerSizeLimit(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	recorder := httptest.NewRecorder()
	err = signer.Set(recorder, map[string]any{"data": strings.Repeat("x", 4096)})
	if !errors.Is(err, cookies.ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
	if values := recorder.Header().Values("Set-Cookie"); len(values) != 0 {
		t.Fatalf("Set-Cookie headers = %q, want none", values)
	}
}

func TestSignerDelete(t *testing.T) {
	signer, err := cookies.NewSigner("session", []string{newKey}, cookies.WithPath("/app"), cookies.WithDomain("example.com"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := signer.Delete(recorder); err != nil {
		t.Fatalf("delete: %v", err)
	}

	header := singleHeader(t, recorder)
	for _, attribute := range []string{"session=;", "Path=/app", "Domain=example.com", "Max-Age=0", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(header, attribute) {
			t.Errorf("Set-Cookie = %q, want %q", header, attribute)
		}
	}
}
