// Package render resolves the HTML values accepted by the framework.
//
// The accepted forms are structural: a value rendering itself with a request
// context, a value rendering itself with only a writer, a string, and a byte
// slice. No HTML component library is imported here.
package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// Renderer is an HTML value that renders itself with a request context.
type Renderer interface {
	Render(context.Context, io.Writer) error
}

type writerRenderer interface {
	Render(io.Writer) error
}

// HTML converts an accepted HTML value to bytes.
//
// Rendering happens into a buffer, so an unsupported value or a render error
// never leaves a partial response behind. Strings and byte slices are trusted
// HTML and copied unchanged.
func HTML(ctx context.Context, value any) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, errors.New("render: HTML value is nil")
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case Renderer:
		var buf bytes.Buffer
		if err := v.Render(ctx, &buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case writerRenderer:
		var buf bytes.Buffer
		if err := v.Render(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("render: unsupported HTML value of type %T", value)
	}
}
