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

func setCookie(t *testing.T, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	if err := cookies.Set(recorder, cookie); err != nil {
		t.Fatalf("set cookie: %v", err)
	}
	return recorder
}

func singleHeader(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	values := recorder.Header().Values("Set-Cookie")
	if len(values) != 1 {
		t.Fatalf("Set-Cookie headers = %q, want exactly one", values)
	}
	return values[0]
}

func cookieValue(t *testing.T, header string) string {
	t.Helper()

	pair := strings.SplitN(header, ";", 2)[0]
	_, value, found := strings.Cut(pair, "=")
	if !found {
		t.Fatalf("header %q has no value", header)
	}
	return value
}

func requestWithCookies(lines ...string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, line := range lines {
		request.Header.Add("Cookie", line)
	}
	return request
}

func TestSetAttributes(t *testing.T) {
	expires := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	recorder := setCookie(t, &http.Cookie{
		Name:     "sid",
		Value:    "abc",
		Path:     "/app",
		Domain:   "example.com",
		Expires:  expires,
		MaxAge:   60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	want := "sid=abc; Path=/app; Domain=example.com; Expires=Wed, 02 Jan 2030 03:04:05 GMT; Max-Age=60; HttpOnly; Secure; SameSite=Strict"
	if got := singleHeader(t, recorder); got != want {
		t.Errorf("Set-Cookie = %q, want %q", got, want)
	}
}

func TestSetMultipleCookies(t *testing.T) {
	recorder := httptest.NewRecorder()
	for _, name := range []string{"one", "two"} {
		if err := cookies.Set(recorder, &http.Cookie{Name: name, Value: "v"}); err != nil {
			t.Fatalf("set %q: %v", name, err)
		}
	}

	values := recorder.Header().Values("Set-Cookie")
	if len(values) != 2 {
		t.Fatalf("Set-Cookie headers = %q, want two", values)
	}
}

func TestSameSiteNoneImpliesSecure(t *testing.T) {
	cookie := &http.Cookie{Name: "sid", Value: "abc", SameSite: http.SameSiteNoneMode}
	recorder := setCookie(t, cookie)

	if header := singleHeader(t, recorder); !strings.Contains(header, "; Secure") {
		t.Errorf("Set-Cookie = %q, want Secure", header)
	}
	if !cookie.Secure {
		t.Error("cookie.Secure = false, want true after Set")
	}
}

func TestSetValidation(t *testing.T) {
	cases := map[string]*http.Cookie{
		"empty name":          {Name: "", Value: "v"},
		"space in name":       {Name: "a b", Value: "v"},
		"semicolon in name":   {Name: "a;b", Value: "v"},
		"comma in name":       {Name: "a,b", Value: "v"},
		"equals in name":      {Name: "a=b", Value: "v"},
		"non-ASCII name":      {Name: "naïve", Value: "v"},
		"attribute name":      {Name: "expires", Value: "v"},
		"attribute name case": {Name: "Path", Value: "v"},
		"attribute max-age":   {Name: "max-age", Value: "v"},
		"control in value":    {Name: "sid", Value: "\x01"},
		"semicolon in value":  {Name: "sid", Value: "a;b"},
		"comma in value":      {Name: "sid", Value: "a,b"},
		"quote in value":      {Name: "sid", Value: `a"b`},
		"backslash in value":  {Name: "sid", Value: `a\b`},
		"semicolon in path":   {Name: "sid", Value: "v", Path: "/a;b"},
		"comma in domain":     {Name: "sid", Value: "v", Domain: "example.com,x"},
		"unknown same-site":   {Name: "sid", Value: "v", SameSite: http.SameSite(99)},
	}
	for name, cookie := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			err := cookies.Set(recorder, cookie)
			if !errors.Is(err, cookies.ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if values := recorder.Header().Values("Set-Cookie"); len(values) != 0 {
				t.Fatalf("Set-Cookie headers = %q, want none", values)
			}
		})
	}
}

func TestSetPrefixRules(t *testing.T) {
	valid := map[string]*http.Cookie{
		"host":             {Name: "__Host-sid", Value: "v", Path: "/", Secure: true},
		"host lowercase":   {Name: "__host-sid", Value: "v", Path: "/", Secure: true},
		"secure":           {Name: "__Secure-sid", Value: "v", Secure: true},
		"host same-site":   {Name: "__Host-sid", Value: "v", Path: "/", SameSite: http.SameSiteNoneMode},
		"secure same-site": {Name: "__Secure-sid", Value: "v", SameSite: http.SameSiteNoneMode},
	}
	for name, cookie := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if err := cookies.Set(httptest.NewRecorder(), cookie); err != nil {
				t.Fatalf("set cookie: %v", err)
			}
		})
	}

	invalid := map[string]*http.Cookie{
		"host without secure": {Name: "__Host-sid", Value: "v", Path: "/"},
		"host wrong path":     {Name: "__Host-sid", Value: "v", Path: "/app", Secure: true},
		"host with domain":    {Name: "__Host-sid", Value: "v", Path: "/", Domain: "example.com", Secure: true},
		"secure without":      {Name: "__Secure-sid", Value: "v"},
		"host case":           {Name: "__HOST-sid", Value: "v", Path: "/app", Secure: true},
	}
	for name, cookie := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			err := cookies.Set(httptest.NewRecorder(), cookie)
			if !errors.Is(err, cookies.ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cookie := &http.Cookie{
		Name:     "sid",
		Value:    "secret",
		Path:     "/app",
		Domain:   "example.com",
		MaxAge:   60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	recorder := httptest.NewRecorder()
	if err := cookies.Delete(recorder, cookie); err != nil {
		t.Fatalf("delete cookie: %v", err)
	}

	want := "sid=; Path=/app; Domain=example.com; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	if got := singleHeader(t, recorder); got != want {
		t.Errorf("Set-Cookie = %q, want %q", got, want)
	}
	if cookie.Value != "secret" || cookie.MaxAge != 60 {
		t.Errorf("Delete changed the caller's cookie: %+v", cookie)
	}

	if err := cookies.Delete(recorder, cookie); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if values := recorder.Header().Values("Set-Cookie"); len(values) != 2 {
		t.Errorf("Set-Cookie headers = %q, want two", values)
	}
}

func TestDeleteValidation(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := cookies.Delete(recorder, &http.Cookie{Name: "", Value: "v"})
	if !errors.Is(err, cookies.ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
	if values := recorder.Header().Values("Set-Cookie"); len(values) != 0 {
		t.Fatalf("Set-Cookie headers = %q, want none", values)
	}
}

func TestRead(t *testing.T) {
	cases := []struct {
		name   string
		lines  []string
		want   map[string]string
		wantNo []string
	}{
		{
			name:  "one line",
			lines: []string{"sid=abc; theme=dark"},
			want:  map[string]string{"sid": "abc", "theme": "dark"},
		},
		{
			name:  "later line wins",
			lines: []string{"sid=first", "sid=second"},
			want:  map[string]string{"sid": "second"},
		},
		{
			name:  "later pair wins",
			lines: []string{"sid=first; sid=second"},
			want:  map[string]string{"sid": "second"},
		},
		{
			name:   "malformed pairs skipped",
			lines:  []string{"bad; good=1; =nope"},
			want:   map[string]string{"good": "1"},
			wantNo: []string{"bad", ""},
		},
		{
			name:  "value with equals",
			lines: []string{"token=a=b"},
			want:  map[string]string{"token": "a=b"},
		},
		{
			name:  "quoted value",
			lines: []string{`q="hello world"`},
			want:  map[string]string{"q": "hello world"},
		},
		{
			name:  "no cookies",
			lines: nil,
			want:  map[string]string{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := cookies.Read(requestWithCookies(testCase.lines...))
			if len(got) != len(testCase.want) {
				t.Fatalf("Read() = %q, want %q", got, testCase.want)
			}
			for name, want := range testCase.want {
				if got[name] != want {
					t.Errorf("Read()[%q] = %q, want %q", name, got[name], want)
				}
			}
			for _, name := range testCase.wantNo {
				if _, found := got[name]; found {
					t.Errorf("Read() holds %q, want it skipped", name)
				}
			}
		})
	}
}
