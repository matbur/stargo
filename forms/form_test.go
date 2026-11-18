package forms_test

import (
	"os"
	"strings"
	"testing"

	"net/http"

	"github.com/matbur/stargo/forms"
)

func TestFormFieldAccess(t *testing.T) {
	form := readForm(t, urlEncodedRequest(http.MethodPost, "tag=a&tag=b"))

	t.Run("first value", func(t *testing.T) {
		if got := form.Get("tag"); got != "a" {
			t.Errorf("Get(tag) = %q, want %q", got, "a")
		}
	})

	t.Run("all values", func(t *testing.T) {
		wantValues(t, form, "tag", "a", "b")
	})

	t.Run("missing field", func(t *testing.T) {
		if got := form.Get("missing"); got != "" {
			t.Errorf("Get(missing) = %q, want an empty string", got)
		}
		if values := form.Values("missing"); len(values) != 0 {
			t.Errorf("Values(missing) = %q, want no values", values)
		}
	})

	t.Run("values copy", func(t *testing.T) {
		values := form.Values("tag")
		values[0] = "changed"
		if got := form.Get("tag"); got != "a" {
			t.Errorf("Get(tag) after changing the copy = %q, want %q", got, "a")
		}
	})
}

func TestFormFileAccess(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "files", file: true, filename: "one.txt", content: "1"},
		part{field: "files", file: true, filename: "two.txt", content: "2"},
	)
	form := readForm(t, request)

	t.Run("first file", func(t *testing.T) {
		file := form.File("files")
		if file == nil {
			t.Fatal("File(files) = nil, want an upload")
		}
		if file.Filename != "one.txt" {
			t.Errorf("Filename = %q, want %q", file.Filename, "one.txt")
		}
	})

	t.Run("all files in order", func(t *testing.T) {
		files := form.Files("files")
		if len(files) != 2 {
			t.Fatalf("len(Files(files)) = %d, want 2", len(files))
		}
		if files[0].Filename != "one.txt" || files[1].Filename != "two.txt" {
			t.Errorf("filenames = %q, %q, want one.txt, two.txt", files[0].Filename, files[1].Filename)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if file := form.File("missing"); file != nil {
			t.Errorf("File(missing) = %+v, want nil", file)
		}
		if files := form.Files("missing"); len(files) != 0 {
			t.Errorf("Files(missing) = %v, want no files", files)
		}
	})
}

func TestUploadFileReadAll(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "upload", file: true, filename: "a.txt", contentType: "text/plain", content: "hello"},
	)
	form := readForm(t, request)

	file := form.File("upload")
	if file == nil {
		t.Fatal("File(upload) = nil, want an upload")
	}
	content, err := file.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("ReadAll() = %q, want %q", content, "hello")
	}
	if file.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", file.Size, len(content))
	}
}

func TestUploadFileOpenAndClose(t *testing.T) {
	request := multipartRequest(t, http.MethodPost,
		part{field: "upload", file: true, filename: "a.txt", content: "hello"},
	)
	form := readForm(t, request)

	file := form.File("upload")
	if file == nil {
		t.Fatal("File(upload) = nil, want an upload")
	}
	reader, err := file.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	content := make([]byte, 5)
	if _, err := reader.Read(content); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestFormCloseIsNilSafe(t *testing.T) {
	var form *forms.Form
	if err := form.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
	if err := new(forms.Form).Close(); err != nil {
		t.Fatalf("empty Close: %v", err)
	}
}

func TestFormCloseRemovesTemporaryFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	request := multipartRequest(t, http.MethodPost,
		part{field: "upload", file: true, filename: "a.bin", content: strings.Repeat("x", 4096)},
	)
	form, err := forms.Read(request, forms.WithMemoryBytes(16))
	if err != nil {
		t.Fatalf("read form: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files before Close = %d, want 1", len(entries))
	}

	if err := form.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	entries, err = os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files after Close = %d, want 0", len(entries))
	}
}

func TestSmallUploadStaysInMemory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	request := multipartRequest(t, http.MethodPost,
		part{field: "upload", file: true, filename: "a.txt", content: "small"},
	)
	form := readForm(t, request, forms.WithMemoryBytes(1<<20))

	if file := form.File("upload"); file == nil || file.Size != int64(len("small")) {
		t.Fatalf("File(upload) = %+v, want a small upload", file)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files = %d, want 0", len(entries))
	}
}
