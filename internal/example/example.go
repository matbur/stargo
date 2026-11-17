// Package example wires the runnable counter application used by
// examples/hello.
package example

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/internal/views"
	"github.com/matbur/stargo/relay"
	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/signals"
	"github.com/matbur/stargo/sse"
)

// Routes declared once and used for registration, span names, and links.
var (
	Home    = route.Get("/")
	Live    = route.Get("/live")
	Command = route.Post("/command")
)

// Subject is the relay subject carrying counter values.
const Subject = "counter"

// New returns the example application.
func New(opts ...stargo.Option) *stargo.App {
	app := stargo.New(opts...)
	hub := relay.New[int64]()
	var count atomic.Int64

	app.Handle(Home, func(c *stargo.Ctx) error {
		return c.HTML(views.Page(count.Load(), Live.Href()))
	})
	app.Handle(Live, live(hub, &count))
	app.Handle(Command, command(hub, &count))
	return app
}

func live(hub *relay.Relay[int64], count *atomic.Int64) stargo.Handler {
	return func(c *stargo.Ctx) error {
		stream, err := c.Stream()
		if err != nil {
			return err
		}
		ctx := c.R.Context()

		subscription, err := hub.Subscribe(ctx, Subject)
		if err != nil {
			return err
		}
		defer subscription.Close()

		if err := stream.PatchElements(
			views.Counter(count.Load()),
			sse.WithSelector("#counter"),
			sse.WithMode(sse.ModeOuter),
		); err != nil {
			return nil // the client is already gone
		}

		// Heartbeat so proxies do not close an idle stream. The stream mutex
		// serializes this goroutine with the patch loop.
		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if err := stream.Ping(""); err != nil {
						return
					}
				}
			}
		}()

		for {
			_, value, err := subscription.Next(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, relay.ErrClosed) {
					return nil
				}
				return err
			}
			if err := stream.PatchElements(
				views.Counter(value),
				sse.WithSelector("#counter"),
				sse.WithMode(sse.ModeOuter),
			); err != nil {
				return nil // disconnected; the span records request.client_left
			}
		}
	}
}

func command(hub *relay.Relay[int64], count *atomic.Int64) stargo.Handler {
	return func(c *stargo.Ctx) error {
		if _, err := signals.Read(c.R); err != nil {
			return stargo.NewHTTPError(400, "invalid signals")
		}
		value := count.Add(1)
		hub.Publish(Subject, value)
		return c.NoContent()
	}
}
