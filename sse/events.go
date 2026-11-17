package sse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/matbur/stargo/internal/render"
)

const (
	eventPatchElements = "event: datastar-patch-elements\n"
	eventPatchSignals  = "event: datastar-patch-signals\n"
	defaultRetry       = 1000
)

// PatchElements sends an element patch. The content accepts the same four
// forms as a full page: a context renderer, a writer renderer, a string, or a
// byte slice.
func (s *Stream) PatchElements(content any, opts ...Option) error {
	options := resolveOptions(opts)
	if err := validateCommon(options); err != nil {
		return err
	}
	if options.Mode == ModeRemove {
		return errors.New("sse: remove mode has no element payload; use Remove")
	}

	html, err := render.HTML(s.ctx, content)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString(eventPatchElements)
	writeMeta(&b, options)
	if options.Mode != "" {
		b.WriteString("data: mode ")
		b.WriteString(string(options.Mode))
		b.WriteByte('\n')
	}
	if options.Selector != "" {
		b.WriteString("data: selector ")
		b.WriteString(options.Selector)
		b.WriteByte('\n')
	}
	if options.Namespace != "" {
		b.WriteString("data: namespace ")
		b.WriteString(string(options.Namespace))
		b.WriteByte('\n')
	}
	if options.UseViewTransition {
		b.WriteString("data: useViewTransition true\n")
	}
	if options.ViewTransitionSelector != "" {
		b.WriteString("data: viewTransitionSelector ")
		b.WriteString(options.ViewTransitionSelector)
		b.WriteByte('\n')
	}
	for _, line := range splitPayload(html) {
		b.WriteString("data: elements ")
		b.Write(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(b.Bytes())
}

// PatchSignals sends a signal patch. Dotted keys expand into nested objects,
// so {"ui.open": true} patches $ui.open.
func (s *Stream) PatchSignals(signals map[string]any, opts ...Option) error {
	options := resolveOptions(opts)
	if err := validateCommon(options); err != nil {
		return err
	}
	if options.Mode != "" || options.Selector != "" || options.Namespace != "" ||
		options.UseViewTransition || options.ViewTransitionSelector != "" {
		return errors.New("sse: element options cannot be used on a signal patch")
	}

	expanded, err := expandSignals(signals)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(expanded)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString(eventPatchSignals)
	writeMeta(&b, options)
	if options.OnlyIfMissing {
		b.WriteString("data: onlyIfMissing true\n")
	}
	for _, line := range splitPayload(payload) {
		b.WriteString("data: signals ")
		b.Write(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(b.Bytes())
}

// ExecuteScript runs developer-authored JavaScript by appending a script
// element to body. The code is sent verbatim; never interpolate untrusted
// input into it.
func (s *Stream) ExecuteScript(code string, opts ...Option) error {
	options := resolveOptions(opts)
	options.Mode = ModeAppend
	options.Selector = "body"
	options.Namespace = ""

	var script string
	if options.AutoRemove {
		script = `<script data-effect="el.remove();">` + code + "</script>"
	} else {
		script = "<script>" + code + "</script>"
	}
	return s.PatchElements(script, optionsToSlice(options)...)
}

// Remove removes nodes matching selector.
func (s *Stream) Remove(selector string, opts ...Option) error {
	options := resolveOptions(opts)
	options.Mode = ModeRemove
	options.Selector = selector
	options.Namespace = ""
	options.UseViewTransition = false
	options.ViewTransitionSelector = ""
	if err := validateCommon(options); err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString(eventPatchElements)
	writeMeta(&b, options)
	b.WriteString("data: mode remove\n")
	b.WriteString("data: selector ")
	b.WriteString(selector)
	b.WriteByte('\n')
	b.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(b.Bytes())
}

// Navigate makes the browser load url from an SSE response.
func (s *Stream) Navigate(url string, opts ...Option) error {
	encoded, err := json.Marshal(url)
	if err != nil {
		return err
	}
	code := "setTimeout(() => window.location = " + string(encoded) + ");"
	return s.ExecuteScript(code, opts...)
}

// Ping writes a comment-only event that keeps intermediaries from closing an
// idle connection.
func (s *Stream) Ping(comment string) error {
	if err := validateField("comment", comment); err != nil {
		return err
	}
	event := []byte(": " + comment + "\n\n")

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(event)
}

func validateCommon(options Options) error {
	if err := options.validateMode(); err != nil {
		return err
	}
	if err := options.validateNamespace(); err != nil {
		return err
	}
	if err := options.validateRetry(); err != nil {
		return err
	}
	if options.Selector != "" {
		if err := validateField("selector", options.Selector); err != nil {
			return err
		}
	}
	if options.ViewTransitionSelector != "" {
		if err := validateField("viewTransitionSelector", options.ViewTransitionSelector); err != nil {
			return err
		}
	}
	if options.EventID != "" {
		if err := validateField("event_id", options.EventID); err != nil {
			return err
		}
		if strings.IndexByte(options.EventID, 0) >= 0 {
			return errors.New("sse: event id must not contain NUL")
		}
	}
	return nil
}

func validateField(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("sse: %s must not contain line breaks", name)
	}
	return nil
}

func writeMeta(b *bytes.Buffer, options Options) {
	if options.EventID != "" {
		b.WriteString("id: ")
		b.WriteString(options.EventID)
		b.WriteByte('\n')
	}
	if options.Retry > 0 && options.Retry != defaultRetry {
		b.WriteString("retry: ")
		b.WriteString(strconv.Itoa(options.Retry))
		b.WriteByte('\n')
	}
}

func splitPayload(payload []byte) [][]byte {
	if bytes.IndexByte(payload, '\r') < 0 {
		return bytes.Split(payload, []byte{'\n'})
	}
	payload = bytes.ReplaceAll(payload, []byte("\r\n"), []byte("\n"))
	payload = bytes.ReplaceAll(payload, []byte("\r"), []byte("\n"))
	return bytes.Split(payload, []byte{'\n'})
}

func optionsToSlice(options Options) []Option {
	return []Option{func(o *Options) { *o = options }}
}

func expandSignals(signals map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(signals))
	for key, value := range signals {
		if key == "" {
			return nil, errors.New("sse: signal name must not be empty")
		}
		parts := strings.Split(key, ".")
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("sse: signal name %q has an empty path segment", key)
			}
		}

		current := out
		for _, part := range parts[:len(parts)-1] {
			next, ok := current[part]
			if !ok {
				child := make(map[string]any)
				current[part] = child
				current = child
				continue
			}
			child, ok := next.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("sse: signal %q conflicts with an existing value", key)
			}
			current = child
		}
		last := parts[len(parts)-1]
		if _, exists := current[last]; exists {
			return nil, fmt.Errorf("sse: signal %q is declared more than once", key)
		}
		current[last] = value
	}
	return out, nil
}
