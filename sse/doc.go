// Package sse writes Datastar Server-Sent Events to one HTTP response.
//
// A Stream is created per response and owns every write to it. Each event is
// encoded into one buffer, written once, and flushed, so the client receives
// complete events as soon as they are produced.
//
// A stream created with WithRequest compresses events with gzip when the
// request accepts it. Ctx.Stream passes the request automatically.
package sse
