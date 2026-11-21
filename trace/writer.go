package trace

import (
	"bufio"
	"encoding/json"
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

// writer is the bounded, batched NDJSON export pipeline behind a Tracer.
type writer struct {
	cfg         config
	path        string
	destination io.Writer
	queue       chan record
	done        chan struct{}
	dropCount   atomic.Uint64

	mu     sync.RWMutex
	closed bool

	file *os.File
	buf  *bufio.Writer
	size int64
}

func newWriter(cfg config, destination target) *writer {
	w := &writer{
		cfg:         cfg,
		path:        destination.path,
		destination: destination.destination,
		queue:       make(chan record, cfg.queueSize),
		done:        make(chan struct{}),
	}

	if w.path != "" {
		file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic("trace: cannot open destination: " + err.Error())
		}
		w.file = file
		if info, err := file.Stat(); err == nil {
			w.size = info.Size()
		}
		w.buf = bufio.NewWriterSize(file, writeBufSize)
	} else {
		w.buf = bufio.NewWriterSize(w.destination, writeBufSize)
	}

	go w.run()
	return w
}

func (w *writer) enqueue(rec record) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		w.dropCount.Add(1)
		return
	}
	select {
	case w.queue <- rec:
	default:
		w.dropCount.Add(1)
	}
}

func (w *writer) dropped() uint64 {
	return w.dropCount.Load()
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

	batch := make([]record, 0, w.cfg.batchSize)
	for {
		select {
		case rec, ok := <-w.queue:
			if !ok {
				w.flush(batch)
				return
			}
			batch = append(batch, rec)
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

func (w *writer) flush(batch []record) {
	if len(batch) == 0 {
		return
	}
	written := 0
	failed := 0
	for _, rec := range batch {
		line, err := json.Marshal(rec)
		if err != nil {
			failed++
			continue
		}
		if _, err := w.buf.Write(line); err != nil {
			failed++
			continue
		}
		if err := w.buf.WriteByte('\n'); err != nil {
			failed++
			continue
		}
		w.size += int64(len(line)) + 1
		written++
	}
	if err := w.buf.Flush(); err != nil {
		failed += written
	}
	if failed > 0 {
		w.dropCount.Add(uint64(failed))
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

	path := w.path
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
