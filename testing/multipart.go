package stargotest

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"sort"
)

// File is one multipart file part.
type File struct {
	// Field is the form field name.
	Field string
	// Name is the filename reported to the server.
	Name string
	// ContentType is the part media type. An empty value becomes
	// application/octet-stream.
	ContentType string
	// Content is the file body.
	Content []byte
}

// Multipart is a multipart/form-data body. Fields may repeat values and Files
// keeps caller order, so two files for one field stay in order.
type Multipart struct {
	Fields url.Values
	Files  []File
}

// encodeMultipart returns the media type and body for form.
func encodeMultipart(form Multipart) (string, []byte, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	names := make([]string, 0, len(form.Fields))
	for name := range form.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, value := range form.Fields[name] {
			if err := writer.WriteField(name, value); err != nil {
				return "", nil, err
			}
		}
	}

	for index, file := range form.Files {
		if file.Field == "" {
			return "", nil, fmt.Errorf("file %d has no field name", index)
		}
		contentType := file.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		header := make(textproto.MIMEHeader)
		header.Set(
			"Content-Disposition",
			fmt.Sprintf(`form-data; name=%q; filename=%q`, file.Field, file.Name),
		)
		header.Set("Content-Type", contentType)

		part, err := writer.CreatePart(header)
		if err != nil {
			return "", nil, err
		}
		if _, err := part.Write(file.Content); err != nil {
			return "", nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return writer.FormDataContentType(), buffer.Bytes(), nil
}
