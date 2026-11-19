package stargotest_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	stargotest "github.com/matbur/stargo/testing"
)

// fakeTB records failures and runs cleanups registered by a client.
//
// Fatalf stops the calling goroutine, like testing.T, so failure paths return
// from the helper exactly as they do in a real test.
type fakeTB struct {
	mu       sync.Mutex
	failures []string
	cleanups []func()
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.mu.Lock()
	f.failures = append(f.failures, fmt.Sprintf(format, args...))
	f.mu.Unlock()
	runtime.Goexit()
}

func (f *fakeTB) Cleanup(cleanup func()) {
	f.mu.Lock()
	f.cleanups = append(f.cleanups, cleanup)
	f.mu.Unlock()
}

func (f *fakeTB) failure() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.failures) == 0 {
		return ""
	}
	return f.failures[0]
}

func (f *fakeTB) runCleanups() {
	f.mu.Lock()
	cleanups := append([]func(){}, f.cleanups...)
	f.mu.Unlock()
	for index := len(cleanups) - 1; index >= 0; index-- {
		cleanups[index]()
	}
}

// captureFailure runs fn with a fake TB and returns the first failure message.
func captureFailure(t *testing.T, fn func(stargotest.TB)) string {
	t.Helper()

	result := make(chan string, 1)
	go func() {
		fake := &fakeTB{}
		defer func() {
			fake.runCleanups()
			result <- fake.failure()
		}()
		fn(fake)
	}()
	return <-result
}
