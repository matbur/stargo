package trace

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"time"
	"uuid"
)

const (
	otlpKindInternal = 1
	otlpKindServer   = 2
	otlpStatusError  = 2

	otlpScopeName    = "github.com/matbur/stargo/trace"
	defaultService   = "stargo"
	defaultTraceKind = otlpKindInternal
)

// OTLP/JSON structures. Field names follow the OpenTelemetry protocol JSON
// mapping, including string-encoded 64-bit integers.
type otlpPayload struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource     `json:"resource"`
	ScopeSpans []otlpScopeSpans `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpScopeSpans struct {
	Scope otlpScope  `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}

type otlpScope struct {
	Name string `json:"name"`
}

type otlpSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId,omitempty"`
	Name              string         `json:"name"`
	Kind              int            `json:"kind"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	EndTimeUnixNano   string         `json:"endTimeUnixNano"`
	Attributes        []otlpKeyValue `json:"attributes,omitempty"`
	Events            []otlpEvent    `json:"events,omitempty"`
	Status            *otlpStatus    `json:"status,omitempty"`
}

type otlpEvent struct {
	TimeUnixNano string         `json:"timeUnixNano"`
	Name         string         `json:"name"`
	Attributes   []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type otlpKeyValue struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpValue struct {
	StringValue *string          `json:"stringValue,omitempty"`
	BoolValue   *bool            `json:"boolValue,omitempty"`
	IntValue    *string          `json:"intValue,omitempty"`
	DoubleValue *float64         `json:"doubleValue,omitempty"`
	ArrayValue  *otlpArrayValue  `json:"arrayValue,omitempty"`
	KvlistValue *otlpKvlistValue `json:"kvlistValue,omitempty"`
}

type otlpArrayValue struct {
	Values []otlpValue `json:"values"`
}

type otlpKvlistValue struct {
	Values []otlpKeyValue `json:"values"`
}

func buildOTLPPayload(records []record, serviceName string) otlpPayload {
	spans := make([]otlpSpan, 0, len(records))
	for _, rec := range records {
		spans = append(spans, otlpSpanOf(rec))
	}
	return otlpPayload{
		ResourceSpans: []otlpResourceSpans{{
			Resource: otlpResourceOf(serviceName),
			ScopeSpans: []otlpScopeSpans{{
				Scope: otlpScope{Name: otlpScopeName},
				Spans: spans,
			}},
		}},
	}
}

func otlpSpanOf(rec record) otlpSpan {
	span := otlpSpan{
		TraceID:           traceIDHex(rec.TraceID),
		SpanID:            spanIDHex(rec.SpanID),
		ParentSpanID:      spanIDHex(rec.ParentID),
		Name:              rec.Name,
		Kind:              otlpKind(rec.Kind),
		StartTimeUnixNano: strconv.FormatInt(rec.StartNS, 10),
		EndTimeUnixNano:   strconv.FormatInt(rec.EndNS, 10),
		Attributes:        otlpAttributes(rec.Attributes),
	}
	if rec.Status == "error" {
		span.Status = &otlpStatus{Code: otlpStatusError, Message: rec.Error}
	}
	for _, event := range rec.Events {
		span.Events = append(span.Events, otlpEvent{
			TimeUnixNano: strconv.FormatInt(event.TimeNS, 10),
			Name:         event.Name,
			Attributes:   otlpAttributes(event.Attributes),
		})
	}
	return span
}

func otlpKind(kind string) int {
	switch kind {
	case "request", "stream":
		return otlpKindServer
	case "startup":
		return otlpKindInternal
	default:
		return defaultTraceKind
	}
}

func traceIDHex(value string) string {
	id, err := uuid.Parse(value)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(id[:])
}

func spanIDHex(value string) string {
	id, err := uuid.Parse(value)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(id[8:])
}

func otlpResourceOf(serviceName string) otlpResource {
	attributes := map[string]any{
		"service.name":            serviceNameOrDefault(serviceName),
		"process.runtime.name":    "go",
		"process.runtime.version": runtime.Version(),
		"telemetry.sdk.name":      "stargo",
		"telemetry.sdk.language":  "go",
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		attributes["host.name"] = host
	}
	if version := serviceVersion(); version != "" {
		attributes["service.version"] = version
	}
	return otlpResource{Attributes: otlpAttributes(attributes)}
}

func serviceNameOrDefault(name string) string {
	if name == "" {
		return defaultService
	}
	return name
}

func serviceVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}

func otlpAttributes(attributes map[string]any) []otlpKeyValue {
	if len(attributes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([]otlpKeyValue, 0, len(keys))
	for _, key := range keys {
		values = append(values, otlpKeyValue{Key: key, Value: otlpValueOf(attributes[key])})
	}
	return values
}

func otlpValueOf(value any) otlpValue {
	switch typed := value.(type) {
	case nil:
		return otlpString("")
	case string:
		return otlpString(typed)
	case bool:
		return otlpBool(typed)
	case int:
		return otlpInt(strconv.FormatInt(int64(typed), 10))
	case int8:
		return otlpInt(strconv.FormatInt(int64(typed), 10))
	case int16:
		return otlpInt(strconv.FormatInt(int64(typed), 10))
	case int32:
		return otlpInt(strconv.FormatInt(int64(typed), 10))
	case int64:
		return otlpInt(strconv.FormatInt(typed, 10))
	case uint:
		return otlpInt(strconv.FormatUint(uint64(typed), 10))
	case uint8:
		return otlpInt(strconv.FormatUint(uint64(typed), 10))
	case uint16:
		return otlpInt(strconv.FormatUint(uint64(typed), 10))
	case uint32:
		return otlpInt(strconv.FormatUint(uint64(typed), 10))
	case uint64:
		return otlpInt(strconv.FormatUint(typed, 10))
	case float32:
		return otlpDouble(float64(typed))
	case float64:
		return otlpDouble(typed)
	case time.Duration:
		return otlpInt(strconv.FormatInt(int64(typed), 10))
	case []string:
		values := make([]otlpValue, 0, len(typed))
		for _, item := range typed {
			values = append(values, otlpValueOf(item))
		}
		return otlpValue{ArrayValue: &otlpArrayValue{Values: values}}
	case []any:
		values := make([]otlpValue, 0, len(typed))
		for _, item := range typed {
			values = append(values, otlpValueOf(item))
		}
		return otlpValue{ArrayValue: &otlpArrayValue{Values: values}}
	case map[string]any:
		return otlpValue{KvlistValue: &otlpKvlistValue{Values: otlpAttributes(typed)}}
	case map[string]string:
		converted := make(map[string]any, len(typed))
		for key, item := range typed {
			converted[key] = item
		}
		return otlpValue{KvlistValue: &otlpKvlistValue{Values: otlpAttributes(converted)}}
	case error:
		return otlpString(typed.Error())
	case fmt.Stringer:
		return otlpString(typed.String())
	default:
		return otlpString(fmt.Sprint(typed))
	}
}

func otlpString(value string) otlpValue {
	return otlpValue{StringValue: &value}
}

func otlpBool(value bool) otlpValue {
	return otlpValue{BoolValue: &value}
}

func otlpInt(value string) otlpValue {
	return otlpValue{IntValue: &value}
}

func otlpDouble(value float64) otlpValue {
	return otlpValue{DoubleValue: &value}
}
