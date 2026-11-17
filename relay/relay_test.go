package relay_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/matbur/stargo/relay"
)

func TestMultipleSubscribers(t *testing.T) {
	t.Parallel()

	hub := relay.New[string]()
	first, err := hub.Subscribe(context.Background(), "room.7.message")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := hub.Subscribe(context.Background(), "room.7.*")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	hub.Publish("room.7.message", "hello")

	for name, sub := range map[string]*relay.Subscription[string]{"first": first, "second": second} {
		subject, value, err := sub.Next(context.Background())
		if err != nil {
			t.Fatalf("%s: Next() error = %v", name, err)
		}
		if subject != "room.7.message" || value != "hello" {
			t.Fatalf("%s: got %q=%q", name, subject, value)
		}
	}
}

func TestPatternMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		subject string
		match   bool
	}{
		{"room.7.message", "room.7.message", true},
		{"room.7.message", "room.7.other", false},
		{"room.7.*", "room.7.typing", true},
		{"room.7.*", "room.7.deeper.typing", true},
		{"room.7.*", "room.8.typing", false},
		{"ab.*", "abc.x", false},
		{"ab.*", "ab.x", true},
		{"*", "anything.at.all", true},
	}

	for _, test := range tests {
		hub := relay.New[int]()
		sub, err := hub.Subscribe(context.Background(), test.pattern)
		if err != nil {
			t.Fatalf("Subscribe(%q) error = %v", test.pattern, err)
		}

		hub.Publish(test.subject, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, _, nextErr := sub.Next(ctx)
		cancel()
		sub.Close()

		gotMatch := nextErr == nil
		if gotMatch != test.match {
			t.Fatalf("pattern %q subject %q: match = %v, want %v (err %v)",
				test.pattern, test.subject, gotMatch, test.match, nextErr)
		}
	}
}

func TestCoalescing(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	sub, err := hub.Subscribe(context.Background(), "*")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	hub.Publish("a", 1)
	hub.Publish("a", 2)

	subject, value, err := sub.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if subject != "a" || value != 2 {
		t.Fatalf("got %q=%d, want a=2", subject, value)
	}

	hub.Publish("b", 10)
	hub.Publish("c", 20)

	seen := map[string]int{}
	for i := 0; i < 2; i++ {
		subject, value, err := sub.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		seen[subject] = value
	}
	if seen["b"] != 10 || seen["c"] != 20 {
		t.Fatalf("seen = %v", seen)
	}
}

func TestMaxSizeDropsOldest(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	sub, err := hub.Subscribe(context.Background(), "*", relay.MaxSize(1))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	hub.Publish("a", 1)
	hub.Publish("b", 2)

	subject, value, err := sub.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if subject != "b" || value != 2 {
		t.Fatalf("got %q=%d, want b=2", subject, value)
	}
	if sub.Dropped() != 1 {
		t.Fatalf("dropped = %d, want 1", sub.Dropped())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := sub.Next(ctx); err == nil {
		t.Fatal("expected only one pending subject")
	}
}

func TestLatestOption(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	sub, err := hub.Subscribe(context.Background(), "*", relay.Latest())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	hub.Publish("a", 1)
	hub.Publish("b", 2)

	subject, value, err := sub.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if subject != "b" || value != 2 {
		t.Fatalf("got %q=%d, want b=2", subject, value)
	}
}

func TestNextCancellation(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	sub, err := hub.Subscribe(context.Background(), "*")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, _, err = sub.Next(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Next() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Next() waited %v", elapsed)
	}
}

func TestCloseUnblocksNext(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	sub, err := hub.Subscribe(context.Background(), "*")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := sub.Next(context.Background())
		done <- err
	}()

	time.Sleep(5 * time.Millisecond)
	sub.Close()

	select {
	case err := <-done:
		if !errors.Is(err, relay.ErrClosed) {
			t.Fatalf("Next() error = %v, want ErrClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Next() did not return after Close")
	}

	hub.Publish("a", 1)
	if _, _, err := sub.Next(context.Background()); !errors.Is(err, relay.ErrClosed) {
		t.Fatalf("closed Next() error = %v", err)
	}
}

func TestNoSubscribersDiscards(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	hub.Publish("a", 1) // must not panic or block
}

func TestInvalidPatterns(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	for _, pattern := range []string{"", "a.*.b", "*x", "a..b", ".a", "a."} {
		if _, err := hub.Subscribe(context.Background(), pattern); err == nil {
			t.Fatalf("Subscribe(%q) did not fail", pattern)
		}
	}
}

func TestCanceledContextAtSubscribe(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hub.Subscribe(ctx, "*"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Subscribe() error = %v", err)
	}
}

func TestConcurrentPublishAndSubscribe(t *testing.T) {
	t.Parallel()

	hub := relay.New[int]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var wait sync.WaitGroup
	for i := 0; i < 4; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := 0; j < 100; j++ {
				hub.Publish("room.1.event", j)
			}
		}()
	}

	var readers sync.WaitGroup
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			sub, err := hub.Subscribe(ctx, "room.*")
			if err != nil {
				return
			}
			defer sub.Close()
			for j := 0; j < 5; j++ {
				if _, _, err := sub.Next(ctx); err != nil {
					return
				}
			}
		}()
	}

	wait.Wait()
	readers.Wait()
}
