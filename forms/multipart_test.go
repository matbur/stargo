package forms_test

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/forms"
)

func TestReadMultipart(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "name", content: "Ada"},
		part{field: "avatar", file: true, filename: "photo.png", contentType: "image/png", content: "picture"},
	)
	form := readForm(t, request)

	if got := form.Get("name"); got != "Ada" {
		t.Errorf("Get(name) = %q, want %q", got, "Ada")
	}
	file := form.File("avatar")
	if file == nil {
		t.Fatal("File(avatar) = nil, want an upload")
	}
	if file.Filename != "photo.png" {
		t.Errorf("Filename = %q, want %q", file.Filename, "photo.png")
	}
	if file.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want %q", file.ContentType, "image/png")
	}
	if file.Size != int64(len("picture")) {
		t.Errorf("Size = %d, want %d", file.Size, len("picture"))
	}
	if got, err := file.ReadAll(); err != nil || string(got) != "picture" {
		t.Errorf("ReadAll() = %q, %v, want %q", got, err, "picture")
	}
}

func TestMultipartBodyLimit(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "name", content: "Ada"},
	)
	_, err := forms.Read(request, forms.WithMaxBytes(16))
	if !errors.Is(err, forms.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}

func TestMultipartEmptyFilename(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "avatar", file: true, filename: "", content: ""},
	)
	form := readForm(t, request)

	if file := form.File("avatar"); file != nil {
		t.Errorf("File(avatar) = %+v, want nil", file)
	}
	if files := form.Files("avatar"); len(files) != 0 {
		t.Errorf("Files(avatar) = %v, want no files", files)
	}
}

func TestMultipartFilenameKeepsBaseName(t *testing.T) {
	cases := map[string]string{
		`C:\Users\ada\photo.png`: "photo.png",
		"/var/tmp/photo.png":     "photo.png",
		"photo.png":              "photo.png",
	}
	for filename, want := range cases {
		t.Run(filename, func(t *testing.T) {
			request := multipartRequest(t, http.MethodPost,
				part{field: "avatar", file: true, filename: filename, content: "x"},
			)
			form := readForm(t, request)

			file := form.File("avatar")
			if file == nil {
				t.Fatal("File(avatar) = nil, want an upload")
			}
			if file.Filename != want {
				t.Errorf("Filename = %q, want %q", file.Filename, want)
			}
		})
	}
}

func TestMultipartMalformedBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not a multipart body"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	_, err := forms.Read(request)
	if !errors.Is(err, forms.ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}

func TestMultipartTextOnly(t *testing.T) {
	// A multipart body without file parts behaves like a URL-encoded body.
	request := multipartRequest(t, http.MethodPost,
		part{field: "tag", content: "a"},
		part{field: "tag", content: "b"},
	)
	form := readForm(t, request)

	wantValues(t, form, "tag", "a", "b")
}

func TestFieldCountLimit(t *testing.T) {
	t.Run("multipart at the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "a", content: "1"},
			part{field: "b", content: "2"},
		)
		form := readForm(t, request, forms.WithMaxFields(2))
		if got := form.Get("b"); got != "2" {
			t.Errorf("Get(b) = %q, want %q", got, "2")
		}
	})

	t.Run("multipart over the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "a", content: "1"},
			part{field: "b", content: "2"},
			part{field: "c", content: "3"},
		)
		_, err := forms.Read(request, forms.WithMaxFields(2))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})

	t.Run("url-encoded over the limit", func(t *testing.T) {
		_, err := forms.Read(urlEncodedRequest(http.MethodPost, "a=1&b=2&c=3"), forms.WithMaxFields(2))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})
}

func TestFieldSizeLimit(t *testing.T) {
	t.Run("multipart over the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost, part{field: "a", content: "abcd"})
		_, err := forms.Read(request, forms.WithMaxFieldBytes(3))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})

	t.Run("url-encoded over the limit", func(t *testing.T) {
		_, err := forms.Read(urlEncodedRequest(http.MethodPost, "a=abcd"), forms.WithMaxFieldBytes(3))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})

	t.Run("at the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost, part{field: "a", content: "abc"})
		form := readForm(t, request, forms.WithMaxFieldBytes(3))
		if got := form.Get("a"); got != "abc" {
			t.Errorf("Get(a) = %q, want %q", got, "abc")
		}
	})
}

func TestFileCountLimit(t *testing.T) {
	t.Run("at the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "one", file: true, filename: "a.txt", content: "a"},
			part{field: "two", file: true, filename: "b.txt", content: "b"},
		)
		form := readForm(t, request, forms.WithMaxFiles(2))
		if form.File("two") == nil {
			t.Error("File(two) = nil, want an upload")
		}
	})

	t.Run("over the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "one", file: true, filename: "a.txt", content: "a"},
			part{field: "two", file: true, filename: "b.txt", content: "b"},
		)
		_, err := forms.Read(request, forms.WithMaxFiles(1))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})
}

func TestFileSizeLimit(t *testing.T) {
	t.Run("over the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "upload", file: true, filename: "a.txt", content: "abcdef"},
		)
		_, err := forms.Read(request, forms.WithMaxFileBytes(4))
		if !errors.Is(err, forms.ErrTooLarge) {
			t.Fatalf("error = %v, want ErrTooLarge", err)
		}
	})

	t.Run("at the limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "upload", file: true, filename: "a.txt", content: "abcd"},
		)
		form := readForm(t, request, forms.WithMaxFileBytes(4))
		if form.File("upload") == nil {
			t.Error("File(upload) = nil, want an upload")
		}
	})

	t.Run("zero means no separate limit", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost,
			part{field: "upload", file: true, filename: "a.txt", content: strings.Repeat("x", 1024)},
		)
		form := readForm(t, request, forms.WithMaxFileBytes(0))
		if file := form.File("upload"); file == nil || file.Size != 1024 {
			t.Errorf("File(upload) = %+v, want a 1024-byte upload", file)
		}
	})
}

func TestManyFilesAndFields(t *testing.T) {
	// A realistic browser submission: several fields and file inputs, some
	// left empty.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range []string{"title", "description"} {
		if err := writer.WriteField(name, "value of "+name); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	for index := range 3 {
		fileWriter, err := writer.CreateFormFile("files", fmt.Sprintf("file-%d.txt", index))
		if err != nil {
			t.Fatalf("create file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(strings.Repeat("data", index+1))); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	form := readForm(t, request)

	if got := len(form.Files("files")); got != 3 {
		t.Errorf("len(Files(files)) = %d, want 3", got)
	}
	if got := form.Get("title"); got != "value of title" {
		t.Errorf("Get(title) = %q, want %q", got, "value of title")
	}
}
