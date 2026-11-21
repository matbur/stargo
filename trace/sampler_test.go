package trace

import (
	"errors"
	"testing"
	"time"
	"uuid"
)

func TestTraceIDRatioDeterministic(t *testing.T) {
	t.Parallel()

	zero := uuid.UUID{}
	maximum := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	if TraceIDRatio(0).Sample(zero, SpanContext{}, false) {
		t.Fatal("ratio 0 sampled a trace")
	}
	if !TraceIDRatio(1).Sample(zero, SpanContext{}, false) {
		t.Fatal("ratio 1 did not sample a trace")
	}

	sampler := TraceIDRatio(0.5)
	if !sampler.Sample(zero, SpanContext{}, false) {
		t.Fatal("ratio 0.5 did not sample the empty trace id")
	}
	if sampler.Sample(maximum, SpanContext{}, false) {
		t.Fatal("ratio 0.5 sampled the maximum trace id")
	}
	for range 10 {
		if got := sampler.Sample(zero, SpanContext{}, false); !got {
			t.Fatal("ratio sampler is not deterministic")
		}
	}
}

func TestParentBasedSampler(t *testing.T) {
	t.Parallel()

	off := ParentBased(AlwaysOff())
	if off.Sample(uuid.UUID{}, SpanContext{}, false) {
		t.Fatal("root delegation did not use the always-off sampler")
	}
	if !off.Sample(uuid.UUID{}, SpanContext{Sampled: true}, true) {
		t.Fatal("sampled parent was not followed")
	}
	if off.Sample(uuid.UUID{}, SpanContext{Sampled: false}, true) {
		t.Fatal("unsampled parent was not followed")
	}
}

func TestAlwaysOffSkipsExport(t *testing.T) {
	t.Parallel()

	buffer := &lockedBuffer{}
	tracer := New(
		Destination(buffer),
		WithSampler(AlwaysOff()),
		BatchSize(1),
		FlushInterval(time.Millisecond),
	)
	span := tracer.Start("request", "request")
	if span.ID() == uuid.Nil() {
		t.Fatal("unsampled span has no id")
	}
	child, err := span.Step("child")
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	span.Attr("http.method", "GET")
	span.Event("hidden", nil)
	span.Fail(errors.New("hidden"))
	child.End()
	span.End()
	if err := tracer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := buffer.String(); got != "" {
		t.Fatalf("unsampled spans were exported: %q", got)
	}
}

func TestSamplerFromEnv(t *testing.T) {
	parent := SpanContext{Sampled: true}
	cases := []struct {
		name  string
		arg   string
		root  bool
		child bool
		nil   bool
	}{
		{name: "always_on", root: true, child: true},
		{name: "always_off", root: false, child: false},
		{name: "traceidratio", arg: "0", root: false},
		{name: "traceidratio", arg: "1", root: true},
		{name: "parentbased_always_off", root: false, child: true},
		{name: "parentbased_always_on", root: true, child: true},
		{name: "parentbased_traceidratio", arg: "1", root: true, child: true},
		{name: "unknown", nil: true},
	}
	for _, test := range cases {
		t.Run(test.name+"_"+test.arg, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", test.name)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", test.arg)
			sampler := samplerFromEnv()
			if test.nil {
				if sampler != nil {
					t.Fatal("unknown sampler did not return nil")
				}
				return
			}
			if sampler == nil {
				t.Fatal("samplerFromEnv() returned nil")
			}
			traceID := uuid.UUID{}
			if got := sampler.Sample(traceID, SpanContext{}, false); got != test.root {
				t.Fatalf("root = %v, want %v", got, test.root)
			}
			if test.child {
				if got := sampler.Sample(traceID, parent, true); !got {
					t.Fatal("sampled parent was not followed")
				}
			}
		})
	}
}
