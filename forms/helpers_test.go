package forms_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/matbur/stargo/forms"
)

// part describes one part of a multipart test body. A part with file set is a
// file part, even when its filename is empty.
type part struct {
	field       string
	filename    string
	file        bool
	contentType string
	content     string
}

func urlEncodedRequest(method, body string) *http.Request {
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func multipartRequest(t *testing.T, method string, parts ...part) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, p := range parts {
		header := textproto.MIMEHeader{}
		disposition := fmt.Sprintf(`form-data; name=%q`, p.field)
		if p.file {
			disposition += fmt.Sprintf(`; filename=%q`, p.filename)
		}
		header.Set("Content-Disposition", disposition)
		if p.contentType != "" {
			header.Set("Content-Type", p.contentType)
		}
		partWriter, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := partWriter.Write([]byte(p.content)); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(method, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func readForm(t *testing.T, request *http.Request, opts ...forms.Option) *forms.Form {
	t.Helper()

	form, err := forms.Read(request, opts...)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	t.Cleanup(func() { _ = form.Close() })
	return form
}

func wantValues(t *testing.T, form *forms.Form, name string, want ...string) {
	t.Helper()

	got := form.Values(name)
	if len(got) != len(want) {
		t.Fatalf("Values(%q) = %q, want %q", name, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Values(%q) = %q, want %q", name, got, want)
		}
	}
}
