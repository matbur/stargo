package trace

import (
	"encoding/binary"
	"math"
	"os"
	"strconv"
	"strings"
	"uuid"
)

// SpanContext is a W3C trace context: the trace id plus the span id that is
// its parent, the sampling decision, and the trace state.
type SpanContext struct {
	TraceID    uuid.UUID
	SpanID     uuid.UUID
	Sampled    bool
	TraceState string
}

// Sampler decides whether a root span is exported. Child spans inherit the
// decision of their parent, and a parent-based sampler follows an incoming
// remote decision.
type Sampler interface {
	Sample(traceID uuid.UUID, parent SpanContext, hasParent bool) bool
}

type samplerFunc func(uuid.UUID, SpanContext, bool) bool

func (f samplerFunc) Sample(traceID uuid.UUID, parent SpanContext, hasParent bool) bool {
	return f(traceID, parent, hasParent)
}

// AlwaysOn samples every trace.
func AlwaysOn() Sampler {
	return samplerFunc(func(uuid.UUID, SpanContext, bool) bool { return true })
}

// AlwaysOff samples no trace.
func AlwaysOff() Sampler {
	return samplerFunc(func(uuid.UUID, SpanContext, bool) bool { return false })
}

// TraceIDRatio samples a deterministic fraction of traces based on the low
// eight bytes of the trace id. A ratio at or below 0 samples nothing; a ratio
// at or above 1 samples everything.
func TraceIDRatio(ratio float64) Sampler {
	switch {
	case ratio <= 0:
		return AlwaysOff()
	case ratio >= 1:
		return AlwaysOn()
	}
	return samplerFunc(func(traceID uuid.UUID, _ SpanContext, _ bool) bool {
		value := binary.BigEndian.Uint64(traceID[8:]) >> 1
		return value < uint64(ratio*float64(math.MaxInt64))
	})
}

// ParentBased follows the remote parent's sampling flag when one exists and
// delegates to root otherwise.
func ParentBased(root Sampler) Sampler {
	if root == nil {
		root = AlwaysOn()
	}
	return samplerFunc(func(traceID uuid.UUID, parent SpanContext, hasParent bool) bool {
		if hasParent {
			return parent.Sampled
		}
		return root.Sample(traceID, parent, false)
	})
}

// WithSampler sets the sampler. A nil sampler is ignored. The default is
// ParentBased(AlwaysOn()).
func WithSampler(sampler Sampler) Option {
	return func(c *config) {
		if sampler != nil {
			c.sampler = sampler
		}
	}
}

func (t *Tracer) sample(traceID uuid.UUID, parent SpanContext, hasParent bool) bool {
	if t == nil || t.sampler == nil {
		return true
	}
	return t.sampler.Sample(traceID, parent, hasParent)
}

func samplerFromEnv() Sampler {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	if name == "" {
		return nil
	}
	ratio := func() float64 {
		value := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
		if value == "" {
			return 1
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) {
			return 1
		}
		return parsed
	}
	switch name {
	case "always_on":
		return AlwaysOn()
	case "always_off":
		return AlwaysOff()
	case "traceidratio":
		return TraceIDRatio(ratio())
	case "parentbased_always_on":
		return ParentBased(AlwaysOn())
	case "parentbased_always_off":
		return ParentBased(AlwaysOff())
	case "parentbased_traceidratio":
		return ParentBased(TraceIDRatio(ratio()))
	default:
		return nil
	}
}
