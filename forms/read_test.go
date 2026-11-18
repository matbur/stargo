package forms_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/forms"
)

func TestReadQueryString(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/?name=Ada&tag=a&tag=b", nil)
			form := readForm(t, request)

			if got := form.Get("name"); got != "Ada" {
				t.Errorf("Get(name) = %q, want %q", got, "Ada")
			}
			wantValues(t, form, "tag", "a", "b")
			if form.File("name") != nil {
				t.Error("query form reported a file")
			}
		})
	}
}

func TestReadBody(t *testing.T) {
	form := readForm(t, urlEncodedRequest(http.MethodPost, "name=Ada&tag=a&tag=b"))

	if got := form.Get("name"); got != "Ada" {
		t.Errorf("Get(name) = %q, want %q", got, "Ada")
	}
	wantValues(t, form, "tag", "a", "b")
}

func TestReadMissingBody(t *testing.T) {
	form := readForm(t, urlEncodedRequest(http.MethodPost, ""))

	if got := form.Get("name"); got != "" {
		t.Errorf("Get(name) = %q, want an empty string", got)
	}
	if values := form.Values("name"); len(values) != 0 {
		t.Errorf("Values(name) = %q, want no values", values)
	}
}

func TestURLEncodedDecoding(t *testing.T) {
	form := readForm(t, urlEncodedRequest(http.MethodPost, "name=Ada+Lovelace%21&tag=a&tag=b"))

	if got := form.Get("name"); got != "Ada Lovelace!" {
		t.Errorf("Get(name) = %q, want %q", got, "Ada Lovelace!")
	}
	wantValues(t, form, "tag", "a", "b")
}

func TestURLEncodedMalformedEscape(t *testing.T) {
	_, err := forms.Read(urlEncodedRequest(http.MethodPost, "name=%zz"))
	if !errors.Is(err, forms.ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}

func TestUnsupportedMediaType(t *testing.T) {
	cases := map[string]*http.Request{
		"json": func() *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
			request.Header.Set("Content-Type", "application/json")
			return request
		}(),
		"missing content type": func() *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Ada"))
			request.Header.Del("Content-Type")
			return request
		}(),
		"plain text": func() *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Ada"))
			request.Header.Set("Content-Type", "text/plain")
			return request
		}(),
		"non UTF-8 charset": func() *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Ada"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=iso-8859-1")
			return request
		}(),
		"multipart without boundary": func() *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Ada"))
			request.Header.Set("Content-Type", "multipart/form-data")
			return request
		}(),
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := forms.Read(request)
			if !errors.Is(err, forms.ErrUnsupportedMediaType) {
				t.Fatalf("error = %v, want ErrUnsupportedMediaType", err)
			}
		})
	}
}

func TestUTF8CharsetAccepted(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Ada"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	form := readForm(t, request)

	if got := form.Get("name"); got != "Ada" {
		t.Errorf("Get(name) = %q, want %q", got, "Ada")
	}
}

func TestBodyLimit(t *testing.T) {
	t.Run("at the limit", func(t *testing.T) {
		form := readForm(t, urlEncodedRequest(http.MethodPost, "a=1"), forms.WithMaxBytes(3))
		if got := form.Get("a"); got != "1" {
			t.Errorf("Get(a) = %q, want %q", got, "1")
		}
	})

	t.Run("one byte over", func(t *testing.T) {
		_, err := forms.Read(urlEncodedRequest(http.MethodPost, "a=11"), forms.WithMaxBytes(3))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})

	t.Run("custom limit accepts a larger body", func(t *testing.T) {
		body := "a=" + strings.Repeat("x", 64)
		form := readForm(t, urlEncodedRequest(http.MethodPost, body), forms.WithMaxBytes(1<<10))
		if got := len(form.Get("a")); got != 64 {
			t.Errorf("len(Get(a)) = %d, want 64", got)
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	cases := []struct {
		name    string
		request *http.Request
		option  forms.Option
		want    error
	}{
		{
			name:    "nil request",
			request: nil,
			want:    forms.ErrMalformed,
		},
		{
			name:    "unsupported media type",
			request: httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")),
			want:    forms.ErrUnsupportedMediaType,
		},
		{
			name:    "malformed body",
			request: urlEncodedRequest(http.MethodPost, "a=%zz"),
			want:    forms.ErrMalformed,
		},
		{
			name:    "too large",
			request: urlEncodedRequest(http.MethodPost, "a=11"),
			option:  forms.WithMaxBytes(3),
			want:    forms.ErrTooLarge,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var (
				form *forms.Form
				err  error
			)
			if testCase.option == nil {
				form, err = forms.Read(testCase.request)
			} else {
				form, err = forms.Read(testCase.request, testCase.option)
			}
			if form != nil {
				t.Fatal("Read returned a form with an error")
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}
