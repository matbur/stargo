package console

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func start() time.Time {
	return time.Date(2026, 11, 17, 18, 20, 40, 0, time.UTC)
}

func TestRenderRequestLine(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(100, false)
	got := renderer.Render(Block{
		Start:    start(),
		Name:     "GET /",
		Status:   200,
		Duration: 965 * time.Microsecond,
	})

	want := "18:20:40 ● GET /  200" + strings.Repeat(" ", 73) + " 965µs"
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestRenderStatusAndFailure(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(100, false)
	tests := []struct {
		name   string
		block  Block
		prefix string
	}{
		{
			name:   "not found",
			block:  Block{Start: start(), Name: "GET /missing", Status: 404},
			prefix: "18:20:40 ● GET /missing  404",
		},
		{
			name:   "failure",
			block:  Block{Start: start(), Name: "GET /boom", Status: 500, Failed: true},
			prefix: "18:20:40 × GET /boom  500",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := renderer.Render(test.block)
			if !strings.HasPrefix(got, test.prefix) {
				t.Fatalf("Render() = %q, want prefix %q", got, test.prefix)
			}
			if lines := strings.Count(got, "\n"); lines != 0 {
				t.Fatalf("Render() has %d extra lines", lines)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		duration time.Duration
		want     string
	}{
		{9900 * time.Nanosecond, "9.9µs"},
		{10 * time.Microsecond, "10µs"},
		{250 * time.Microsecond, "250µs"},
		{1 * time.Millisecond, "1.0ms"},
		{1300 * time.Microsecond, "1.3ms"},
		{150 * time.Millisecond, "150ms"},
		{1 * time.Second, "1.00s"},
		{2500 * time.Millisecond, "2.50s"},
		{61 * time.Second, "1m01s"},
	}
	for _, test := range tests {
		if got := formatDuration(test.duration); got != test.want {
			t.Errorf("formatDuration(%v) = %q, want %q", test.duration, got, test.want)
		}
	}
}

func TestAttributeGroupingAndValues(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(100, false)
	got := renderer.Render(Block{
		Start:    start(),
		Name:     "server.startup",
		Duration: 394 * time.Microsecond,
		Attrs: []Attr{
			{Key: "app.name", Value: "hello world"},
			{Key: "server.graceful_shutdown_timeout", Value: 10 * time.Second},
			{Key: "server.timeout.request_header", Value: 5 * time.Second},
			{Key: "server.timeout.keep_alive", Value: 2 * time.Minute},
			{Key: "server.tls", Value: false},
			{Key: "server.listening", Value: true},
		},
	})

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("got %d lines: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "18:20:40 ● server.startup") {
		t.Fatalf("header = %q", lines[0])
	}
	if !strings.Contains(got, `app.name="hello world"`) {
		t.Fatalf("quoted value missing: %q", got)
	}
	if !strings.Contains(got, "graceful_shutdown_timeout=10s") {
		t.Fatalf("folded key missing: %q", got)
	}
	if !strings.Contains(got, "timeout  request_header=5s keep_alive=2m0s") {
		t.Fatalf("grouped keys missing: %q", got)
	}
	if !strings.Contains(got, "tls=false listening=true") {
		t.Fatalf("boolean values missing: %q", got)
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, strings.Repeat(" ", 11)) {
			t.Fatalf("attribute line not indented: %q", line)
		}
	}
}

func TestShutdownEventLayout(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(100, false)
	got := renderer.Render(Block{
		Start:    start(),
		Name:     "server.shutdown",
		Duration: 477 * time.Microsecond,
		Attrs: []Attr{
			{Key: "server.shutdown.trigger", Value: "expected_stop"},
			{Key: "server.shutdown.open_connections", Value: 2},
			{Key: "server.shutdown.idle_closed", Value: 2},
			{Key: "server.shutdown.force_closed", Value: 0},
		},
		Event: &Event{Name: "server.shutdown.closed", Offset: 328 * time.Microsecond},
	})

	lines := strings.Split(got, "\n")
	event := lines[len(lines)-1]
	if !strings.HasPrefix(event, "         └◦ server.shutdown.closed") {
		t.Fatalf("event line = %q", event)
	}
	if !strings.HasSuffix(event, " +328µs") {
		t.Fatalf("event offset = %q", event)
	}
	if runeLen(event) != 100 {
		t.Fatalf("event width = %d, want 100", runeLen(event))
	}
	for _, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "         │ ") {
			t.Fatalf("attribute rail missing: %q", line)
		}
		if !strings.Contains(line, "shutdown  trigger=") {
			t.Fatalf("grouped shutdown attributes missing: %q", line)
		}
	}
}

func TestColorAddsStylesOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	block := Block{Start: start(), Name: "GET /", Status: 200, Duration: time.Millisecond}
	plain := NewRenderer(100, false).Render(block)
	colored := NewRenderer(100, true).Render(block)

	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain output contains ANSI: %q", plain)
	}
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("colored output has no ANSI: %q", colored)
	}
	if stripANSI(colored) != plain {
		t.Fatalf("colored content differs: %q vs %q", stripANSI(colored), plain)
	}
}

func TestWidthCap(t *testing.T) {
	t.Parallel()

	if got := NewRenderer(40, false).Width(); got != 40 {
		t.Fatalf("width = %d, want 40", got)
	}
	if got := NewRenderer(140, false).Width(); got != 100 {
		t.Fatalf("width = %d, want 100", got)
	}
	if got := NewRenderer(0, false).Width(); got != 100 {
		t.Fatalf("default width = %d, want 100", got)
	}
}

func TestWriterWritesWholeBlocks(t *testing.T) {
	t.Parallel()

	var buffer syncBuffer
	writer := NewWriter(&buffer, 100, false)
	block := Block{
		Start:    start(),
		Name:     "GET /",
		Status:   200,
		Duration: time.Millisecond,
		Attrs:    []Attr{{Key: "request.id", Value: "x"}},
	}

	const count = 40
	var group sync.WaitGroup
	group.Add(count)
	for range count {
		go func() {
			defer group.Done()
			writer.Write(block)
		}()
	}
	group.Wait()

	lines := strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n")
	if len(lines) != count*2 {
		t.Fatalf("got %d lines, want %d", len(lines), count*2)
	}
	for i, line := range lines {
		if i%2 == 0 && !strings.HasPrefix(line, "18:20:40 ● GET /  200") {
			t.Fatalf("line %d is not a block header: %q", i, line)
		}
		if i%2 == 1 && !strings.HasPrefix(line, strings.Repeat(" ", 11)) {
			t.Fatalf("line %d is not an attribute: %q", i, line)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func stripANSI(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '\x1b' {
			out.WriteByte(text[i])
			continue
		}
		if i+1 < len(text) && text[i+1] == '[' {
			i += 2
			for i < len(text) && !((text[i] >= 'A' && text[i] <= 'Z') || (text[i] >= 'a' && text[i] <= 'z')) {
				i++
			}
		}
	}
	return out.String()
}
