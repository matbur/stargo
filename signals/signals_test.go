package signals_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/signals"
)

func TestTransports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get query", http.MethodGet, "/?datastar=%7B%22a%22%3A1%7D", ""},
		{"head query", http.MethodHead, "/?datastar=%7B%22a%22%3A1%7D", ""},
		{"delete query", http.MethodDelete, "/?datastar=%7B%22a%22%3A1%7D", ""},
		{"post body", http.MethodPost, "/", `{"a":1}`},
		{"put body", http.MethodPut, "/", `{"a":1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			got, err := signals.Read(request)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if len(got) != 1 || got["a"] != float64(1) {
				t.Fatalf("Read() = %v", got)
			}
		})
	}
}

func TestEmptyPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		target string
	}{
		{"get without query", http.MethodGet, "/"},
		{"post without body", http.MethodPost, "/"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(test.method, test.target, nil)
			got, err := signals.Read(request)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if got == nil || len(got) != 0 {
				t.Fatalf("Read() = %v, want empty map", got)
			}
		})
	}
}

func TestSizeLimit(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"0123456789"}`))
	got, err := signals.Read(request, signals.MaxSize(5))
	if !errors.Is(err, signals.ErrTooLarge) {
		t.Fatalf("Read() error = %v, want ErrTooLarge", err)
	}
	if got != nil {
		t.Fatalf("Read() = %v, want nil on error", got)
	}
}

func TestDefaultSizeLimit(t *testing.T) {
	t.Parallel()

	payload := `{"a":"` + strings.Repeat("x", int(signals.DefaultMaxSize)) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	if _, err := signals.Read(request); !errors.Is(err, signals.ErrTooLarge) {
		t.Fatalf("Read() error = %v, want ErrTooLarge", err)
	}
}

func TestMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		method  string
		target  string
	}{
		{"invalid json", `{"a":`, http.MethodPost, "/"},
		{"array", `[]`, http.MethodPost, "/"},
		{"string", `"x"`, http.MethodPost, "/"},
		{"null", `null`, http.MethodPost, "/"},
		{"invalid query json", `{"a":`, http.MethodGet, "/?datastar=%7B%22a%22%3A"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.payload))
			got, err := signals.Read(request)
			if !errors.Is(err, signals.ErrInvalid) {
				t.Fatalf("Read() error = %v, want ErrInvalid", err)
			}
			if got != nil {
				t.Fatalf("Read() = %v, want nil", got)
			}
		})
	}
}

func TestValuesAreNotValidated(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"count":"not a number","nested":{"deep":true},"local_":1}`))
	got, err := signals.Read(request)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got["count"] != "not a number" {
		t.Fatalf("count = %v", got["count"])
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok || nested["deep"] != true {
		t.Fatalf("nested = %v", got["nested"])
	}
	if got["local_"] != float64(1) {
		t.Fatalf("local_ = %v", got["local_"])
	}
}
