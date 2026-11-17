// Package stargo is a small web framework for Go built on net/http.
//
// An application is a set of handlers, each taking a request context and
// returning an error. The framework owns the response path: it maps errors to
// statuses, recovers panics, traces every request, and serves live pages over
// Server-Sent Events.
package stargo
