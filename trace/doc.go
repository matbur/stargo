// Package trace records request spans as newline-delimited JSON.
//
// Tracing is off by default. When enabled, every request is a span in a tree
// identified by trace_id and parent_id, and finished spans are appended to a
// destination as one JSON object per line.
package trace
