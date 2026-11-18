package forms

import (
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"
	"sync"
)

// Form holds the text fields and uploaded files of one form submission.
//
// Values and filenames are untrusted: validate them before use. A form returned
// for GET or HEAD holds query values and no files. Close the form when done to
// release temporary upload files.
type Form struct {
	values    map[string][]string
	files     map[string][]*UploadFile
	multipart *multipart.Form
}

// Get returns the first value of name, or an empty string when name is absent.
func (f *Form) Get(name string) string {
	if f == nil {
		return ""
	}
	if values := f.values[name]; len(values) > 0 {
		return values[0]
	}
	return ""
}

// Values returns every value of name in submission order, or nil when name is
// absent. The result is a copy; changing it does not change the form.
func (f *Form) Values(name string) []string {
	if f == nil {
		return nil
	}
	values := f.values[name]
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

// File returns the first upload for name, or nil when name has no files.
func (f *Form) File(name string) *UploadFile {
	if f == nil {
		return nil
	}
	files := f.files[name]
	if len(files) == 0 {
		return nil
	}
	return files[0]
}

// Files returns every upload for name in submission order, or nil when name has
// no files. The result is a copy; changing it does not change the form.
func (f *Form) Files(name string) []*UploadFile {
	if f == nil {
		return nil
	}
	files := f.files[name]
	if len(files) == 0 {
		return nil
	}
	return append([]*UploadFile(nil), files...)
}

// Close releases temporary upload files. It is safe to call more than once and
// on a nil form.
func (f *Form) Close() error {
	if f == nil || f.multipart == nil {
		return nil
	}
	multipartForm := f.multipart
	f.multipart = nil
	return multipartForm.RemoveAll()
}

// UploadFile is one file part of a multipart form.
type UploadFile struct {
	// Filename is the client's file name without any directory path. It is
	// untrusted: sanitize it before using it as a path.
	Filename string
	// ContentType is the content type the client declared for the part.
	ContentType string
	// Size is the number of bytes in the upload.
	Size int64

	header *multipart.FileHeader

	mutex sync.Mutex
	open  multipart.File
}

func newUploadFile(header *multipart.FileHeader) *UploadFile {
	return &UploadFile{
		Filename:    baseName(header.Filename),
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		header:      header,
	}
}

func uploadFiles(headers map[string][]*multipart.FileHeader) map[string][]*UploadFile {
	if len(headers) == 0 {
		return nil
	}
	files := make(map[string][]*UploadFile, len(headers))
	for name, list := range headers {
		uploads := make([]*UploadFile, 0, len(list))
		for _, header := range list {
			uploads = append(uploads, newUploadFile(header))
		}
		files[name] = uploads
	}
	return files
}

// Open opens the upload for reading. The caller closes the returned file, or
// calls UploadFile.Close to close the most recently opened one. Opening the same
// upload twice closes the previous file first.
func (u *UploadFile) Open() (multipart.File, error) {
	if u == nil || u.header == nil {
		return nil, ErrNoFile
	}
	file, err := u.header.Open()
	if err != nil {
		return nil, err
	}
	u.mutex.Lock()
	previous := u.open
	u.open = file
	u.mutex.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	return file, nil
}

// ReadAll reads the whole upload into memory. Prefer Open for large files.
func (u *UploadFile) ReadAll() ([]byte, error) {
	if u == nil || u.header == nil {
		return nil, ErrNoFile
	}
	file, err := u.header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// Close closes the file most recently returned by Open, if any. It is safe to
// call more than once and on a nil upload.
func (u *UploadFile) Close() error {
	if u == nil {
		return nil
	}
	u.mutex.Lock()
	file := u.open
	u.open = nil
	u.mutex.Unlock()
	if file == nil {
		return nil
	}
	return file.Close()
}

func (f *Form) checkLimits(config config) error {
	if err := checkValues(f.values, config); err != nil {
		return err
	}
	total := 0
	for _, files := range f.files {
		for _, file := range files {
			total++
			if total > config.maxFiles {
				return fmt.Errorf("%w: more than %d files", ErrTooLarge, config.maxFiles)
			}
			if config.maxFileBytes > 0 && file.Size > config.maxFileBytes {
				return fmt.Errorf("%w: file %q exceeds %d bytes", ErrTooLarge, file.Filename, config.maxFileBytes)
			}
		}
	}
	return nil
}

func baseName(name string) string {
	if name == "" {
		return ""
	}
	// Some clients send a full path, including a Windows one: keep the last
	// part only.
	return path.Base(strings.ReplaceAll(name, "\\", "/"))
}
