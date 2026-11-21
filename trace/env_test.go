package trace

import (
	"testing"
	"time"
)

func clearTracingEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TRACE",
		"TRACE_FILE",
		"TRACE_MAX_MB",
		"TRACE_KEEP",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_HEADERS",
		"OTEL_SERVICE_NAME",
		"OTEL_TRACES_SAMPLER",
		"OTEL_TRACES_SAMPLER_ARG",
	} {
		t.Setenv(key, "")
	}
}

func TestFromEnvOTLPBaseEndpoint(t *testing.T) {
	clearTracingEnv(t)

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_SERVICE_NAME", "env-service")

	tracer := FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("env", "request").End()

	select {
	case request := <-requests:
		if request.path != "/v1/traces" {
			t.Fatalf("path = %q, want /v1/traces", request.path)
		}
		if got := resourceAttribute(request.payload.ResourceSpans[0].Resource, "service.name"); got != "env-service" {
			t.Fatalf("service.name = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector received no request")
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFromEnvExactTracesEndpoint(t *testing.T) {
	clearTracingEnv(t)

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", server.URL+"/collector/custom")

	tracer := FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("env", "request").End()

	select {
	case request := <-requests:
		if request.path != "/collector/custom" {
			t.Fatalf("path = %q, want /collector/custom", request.path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector received no request")
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFromEnvOTLPHeaders(t *testing.T) {
	clearTracingEnv(t)

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "X-A=one, X-B=two%20words")

	tracer := FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("env", "request").End()

	select {
	case request := <-requests:
		if got := request.header.Get("X-A"); got != "one" {
			t.Fatalf("X-A = %q", got)
		}
		if got := request.header.Get("X-B"); got != "two words" {
			t.Fatalf("X-B = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector received no request")
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFromEnvSampler(t *testing.T) {
	clearTracingEnv(t)

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")

	tracer := FromEnv()
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	span := tracer.Start("env", "request")
	if span.Context().Sampled {
		t.Fatal("always_off sampler sampled a span")
	}
	span.End()
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case request := <-requests:
		t.Fatalf("unsampled span was exported: %+v", request)
	default:
	}
}

func TestFromEnvOptionsWin(t *testing.T) {
	clearTracingEnv(t)

	requests := make(chan otlpRequest, 1)
	server := collector(t, requests)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_SERVICE_NAME", "from-env")

	tracer := FromEnv(OTLP(server.URL), ServiceName("explicit"))
	if tracer == nil {
		t.Fatal("FromEnv() returned nil")
	}
	tracer.Start("env", "request").End()

	select {
	case request := <-requests:
		if got := resourceAttribute(request.payload.ResourceSpans[0].Resource, "service.name"); got != "explicit" {
			t.Fatalf("service.name = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("explicit endpoint did not win over the environment")
	}
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
