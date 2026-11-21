package trace

// sink is one export destination owned by a Tracer.
//
// A tracer may own several sinks, for example a local NDJSON writer and an
// OTLP exporter. Each sink owns its own queue so a slow destination cannot
// delay a fast one.
type sink interface {
	enqueue(record)
	close() error
	dropped() uint64
}
