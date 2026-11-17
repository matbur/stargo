package console

import (
	"io"
	"sync"
)

// Writer serializes rendered blocks to one destination.
//
// Each block is issued as a single Write so concurrent requests cannot
// interleave their lines.
type Writer struct {
	mu       sync.Mutex
	out      io.Writer
	renderer *Renderer
}

// NewWriter returns a Writer rendering with width and optional color.
func NewWriter(out io.Writer, width int, color bool) *Writer {
	return &Writer{out: out, renderer: NewRenderer(width, color)}
}

// Write renders one block and writes it with a trailing newline.
func (w *Writer) Write(block Block) {
	if w == nil || w.out == nil {
		return
	}
	text := w.renderer.Render(block) + "\n"
	w.mu.Lock()
	_, _ = io.WriteString(w.out, text)
	w.mu.Unlock()
}

// Width returns the renderer's effective line width.
func (w *Writer) Width() int {
	if w == nil {
		return 0
	}
	return w.renderer.Width()
}
