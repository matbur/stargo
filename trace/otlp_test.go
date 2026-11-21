package trace

import (
	"encoding/hex"
	"testing"
	"time"
	"uuid"
)

func TestOTLPSpanIDsAndParent(t *testing.T) {
	t.Parallel()

	traceID := uuid.NewV7()
	parentID := uuid.NewV7()
	childID := uuid.NewV7()

	payload := buildOTLPPayload([]record{
		{SpanID: parentID.String(), TraceID: traceID.String(), Name: "request", Kind: "request"},
		{SpanID: childID.String(), TraceID: traceID.String(), ParentID: parentID.String(), Name: "db.load"},
	}, "svc")

	spans := payload.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(spans))
	}
	if want := hex.EncodeToString(traceID[:]); spans[0].TraceID != want {
		t.Fatalf("trace id = %q, want %q", spans[0].TraceID, want)
	}
	if len(spans[0].TraceID) != 32 {
		t.Fatalf("trace id length = %d, want 32", len(spans[0].TraceID))
	}
	if want := hex.EncodeToString(parentID[8:]); spans[0].SpanID != want {
		t.Fatalf("span id = %q, want %q", spans[0].SpanID, want)
	}
	if len(spans[0].SpanID) != 16 {
		t.Fatalf("span id length = %d, want 16", len(spans[0].SpanID))
	}
	if spans[1].ParentSpanID != spans[0].SpanID {
		t.Fatalf("child parent = %q, want %q", spans[1].ParentSpanID, spans[0].SpanID)
	}
}

func TestOTLPAttributeValues(t *testing.T) {
	t.Parallel()

	attributes := otlpAttributes(map[string]any{
		"text":    "hello",
		"flag":    true,
		"count":   42,
		"size":    uint64(1 << 63),
		"ratio":   1.5,
		"list":    []any{"a", 1},
		"labels":  map[string]any{"b": "two", "a": "one"},
		"missing": nil,
	})

	byKey := map[string]otlpValue{}
	for _, attribute := range attributes {
		byKey[attribute.Key] = attribute.Value
	}

	if got := byKey["text"].StringValue; got == nil || *got != "hello" {
		t.Fatalf("string value = %+v", byKey["text"])
	}
	if got := byKey["flag"].BoolValue; got == nil || !*got {
		t.Fatalf("bool value = %+v", byKey["flag"])
	}
	if got := byKey["count"].IntValue; got == nil || *got != "42" {
		t.Fatalf("int value = %+v", byKey["count"])
	}
	if got := byKey["size"].IntValue; got == nil || *got != "9223372036854775808" {
		t.Fatalf("uint value = %+v", byKey["size"])
	}
	if got := byKey["ratio"].DoubleValue; got == nil || *got != 1.5 {
		t.Fatalf("double value = %+v", byKey["ratio"])
	}

	list := byKey["list"].ArrayValue
	if list == nil || len(list.Values) != 2 {
		t.Fatalf("array value = %+v", byKey["list"])
	}
	if list.Values[0].StringValue == nil || *list.Values[0].StringValue != "a" {
		t.Fatalf("array string = %+v", list.Values[0])
	}
	if list.Values[1].IntValue == nil || *list.Values[1].IntValue != "1" {
		t.Fatalf("array int = %+v", list.Values[1])
	}

	kvlist := byKey["labels"].KvlistValue
	if kvlist == nil || len(kvlist.Values) != 2 {
		t.Fatalf("kvlist value = %+v", byKey["labels"])
	}
	if kvlist.Values[0].Key != "a" || kvlist.Values[1].Key != "b" {
		t.Fatalf("kvlist order = %q, %q", kvlist.Values[0].Key, kvlist.Values[1].Key)
	}

	if got := byKey["missing"].StringValue; got == nil || *got != "" {
		t.Fatalf("nil value = %+v", byKey["missing"])
	}
}

func TestOTLPSpanMapping(t *testing.T) {
	t.Parallel()

	payload := buildOTLPPayload([]record{
		{
			TraceID:    uuid.NewV7().String(),
			SpanID:     uuid.NewV7().String(),
			Name:       "GET /rooms",
			Kind:       "stream",
			StartNS:    100,
			EndNS:      250,
			Status:     "error",
			Error:      "boom",
			Attributes: map[string]any{"http.status": 500},
			Events: []event{{
				TimeNS:     200,
				Name:       "exception",
				Attributes: map[string]any{"message": "boom"},
			}},
		},
		{TraceID: uuid.NewV7().String(), SpanID: uuid.NewV7().String(), Name: "start", Kind: "startup"},
		{TraceID: uuid.NewV7().String(), SpanID: uuid.NewV7().String(), Name: "other"},
	}, "chat")

	spans := payload.ResourceSpans[0].ScopeSpans[0].Spans
	if got := spans[0].Kind; got != otlpKindServer {
		t.Fatalf("stream kind = %d, want %d", got, otlpKindServer)
	}
	if got := spans[1].Kind; got != otlpKindInternal {
		t.Fatalf("startup kind = %d, want %d", got, otlpKindInternal)
	}
	if got := spans[2].Kind; got != otlpKindInternal {
		t.Fatalf("default kind = %d, want %d", got, otlpKindInternal)
	}

	if spans[0].StartTimeUnixNano != "100" || spans[0].EndTimeUnixNano != "250" {
		t.Fatalf("times = %q, %q", spans[0].StartTimeUnixNano, spans[0].EndTimeUnixNano)
	}
	status := spans[0].Status
	if status == nil || status.Code != otlpStatusError || status.Message != "boom" {
		t.Fatalf("status = %+v", status)
	}
	if len(spans[0].Events) != 1 || spans[0].Events[0].TimeUnixNano != "200" {
		t.Fatalf("events = %+v", spans[0].Events)
	}
	if spans[0].Events[0].Name != "exception" {
		t.Fatalf("event name = %q", spans[0].Events[0].Name)
	}

	resource := payload.ResourceSpans[0].Resource
	if got := resourceAttribute(resource, "service.name"); got != "chat" {
		t.Fatalf("service.name = %q", got)
	}
	if got := resourceAttribute(resource, "process.runtime.name"); got != "go" {
		t.Fatalf("process.runtime.name = %q", got)
	}
	if got := resourceAttribute(resource, "host.name"); got == "" {
		t.Fatal("host.name is missing")
	}
	if got := resourceAttribute(resource, "telemetry.sdk.name"); got != "stargo" {
		t.Fatalf("telemetry.sdk.name = %q", got)
	}
}

func TestOTLPAttributeFallbacks(t *testing.T) {
	t.Parallel()

	value := otlpValueOf(time.Second)
	if value.IntValue == nil || *value.IntValue != "1000000000" {
		t.Fatalf("duration = %+v", value)
	}
	value = otlpValueOf(struct{ Name string }{Name: "x"})
	if value.StringValue == nil || *value.StringValue != "{x}" {
		t.Fatalf("struct fallback = %+v", value)
	}
}

func resourceAttribute(resource otlpResource, key string) string {
	for _, attribute := range resource.Attributes {
		if attribute.Key == key && attribute.Value.StringValue != nil {
			return *attribute.Value.StringValue
		}
	}
	return ""
}
