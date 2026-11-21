// Package trace records request spans and exports them.
//
// Tracing is off by default. When enabled, every request is a span in a tree
// identified by trace_id and parent_id.
//
// Two destinations exist:
//
//   - newline-delimited JSON written to a file, stdout, or an io.Writer;
//   - OTLP/HTTP JSON sent to an OpenTelemetry collector.
//
// A tracer may use both at once. Finished spans are batched through bounded
// queues, so a slow destination never blocks a handler.
//
// A request with a valid W3C traceparent continues that trace, and handlers can
// inject the active context into outgoing requests with Inject. Sampling
// controls export only: ids remain available for propagation.
//
// Local NDJSON keeps full UUID span ids. OTLP uses the full trace id but only
// the last eight bytes of the span id, so a remote parent id in NDJSON is not
// textually identical to the upstream value; trace ids remain comparable.
package trace
