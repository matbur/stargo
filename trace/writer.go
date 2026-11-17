package trace

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	drainTimeout = 5 * time.Second
	writeBufSize = 64 << 10
)

// writer is the bounded, batched export pipeline behind a Tracer.
type writer struct {
	cfg     config
	queue   chan []byte
	done    chan struct{}
	dropped atomic.Uint64

	mu     sync.RWMutex
	closed bool

	file *os.File
	buf  *bufio.Writer
	size int64
}

func newWriter(cfg config) *writer {
	w := &writer{
		cfg:   cfg,
		queue: make(chan []byte, cfg.queueSize),
		done:  make(chan struct{}),
	}

	if cfg.path != "" {
		file, err := os.OpenFile(cfg.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic("trace: cannot open destination: " + err.Error())
		}
		w.file = file
		if info, err := file.Stat(); err == nil {
			w.size = info.Size()
		}
		w.buf = bufio.NewWriterSize(file, writeBufSize)
	} else {
		w.buf = bufio.NewWriterSize(cfg.destination, writeBufSize)
	}

	go w.run()
	return w
}

func (w *writer) enqueue(line []byte) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		w.dropped.Add(1)
		return
	}
	select {
	case w.queue <- line:
	default:
		w.dropped.Add(1)
	}
}

func (w *writer) close() error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.queue)
	}
	w.mu.Unlock()

	select {
	case <-w.done:
	case <-time.After(drainTimeout):
		return errors.New("trace: timed out draining spans")
	}

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *writer) run() {
	defer close(w.done)

	ticker := time.NewTicker(time.Duration(w.cfg.flushInterval))
	defer ticker.Stop()

	batch := make([][]byte, 0, w.cfg.batchSize)
	for {
		select {
		case line, ok := <-w.queue:
			if !ok {
				w.flush(batch)
				return
			}
			batch = append(batch, line)
			if len(batch) >= w.cfg.batchSize {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (w *writer) flush(batch [][]byte) {
	if len(batch) == 0 {
		return
	}
	failed := false
	for _, line := range batch {
		if _, err := w.buf.Write(line); err != nil {
			failed = true
			continue
		}
		if err := w.buf.WriteByte('\n'); err != nil {
			failed = true
			continue
		}
		w.size += int64(len(line)) + 1
	}
	if err := w.buf.Flush(); err != nil {
		failed = true
	}
	if failed {
		w.dropped.Add(uint64(len(batch)))
	}
	if w.cfg.syncWrites && w.file != nil {
		_ = w.file.Sync()
	}
	if w.cfg.rotateBytes > 0 && w.size >= w.cfg.rotateBytes {
		w.rotate()
	}
}

func (w *writer) rotate() {
	if w.file == nil {
		return
	}
	_ = w.buf.Flush()
	_ = w.file.Close()

	path := w.cfg.path
	keep := w.cfg.rotateKeep
	if keep < 1 {
		keep = 1
	}
	if keep == 1 {
		_ = os.Remove(path)
	} else {
		_ = os.Remove(fmt.Sprintf("%s.%d", path, keep-1))
		for i := keep - 2; i >= 1; i-- {
			_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
		}
		_ = os.Rename(path, path+".1")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		w.file = nil
		w.buf = bufio.NewWriterSize(io.Discard, writeBufSize)
		return
	}
	w.file = file
	w.buf = bufio.NewWriterSize(file, writeBufSize)
	w.size = 0
}
