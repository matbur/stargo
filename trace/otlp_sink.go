package trace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultOTLPEndpoint = "http://localhost:4318/v1/traces"
	defaultOTLPTimeout  = 10 * time.Second
	defaultOTLPAttempts = 3
	defaultOTLPRetry    = 100 * time.Millisecond
)

type otlpConfig struct {
	endpoint      string
	exact         bool
	headers       map[string]string
	timeout       time.Duration
	retryAttempts int
	retryDelay    time.Duration
}

func defaultOTLPConfig() otlpConfig {
	return otlpConfig{
		timeout:       defaultOTLPTimeout,
		retryAttempts: defaultOTLPAttempts,
		retryDelay:    defaultOTLPRetry,
	}
}

func ensureOTLP(cfg *config) *otlpConfig {
	if cfg.otlp == nil {
		defaults := defaultOTLPConfig()
		cfg.otlp = &defaults
	}
	return cfg.otlp
}

// OTLP exports spans to an OpenTelemetry collector over OTLP/HTTP JSON.
//
// An empty endpoint uses http://localhost:4318/v1/traces. A non-empty endpoint
// is treated as a base URL and the traces path is appended. The exporter can be
// combined with File, Stdout, or Destination.
func OTLP(endpoint string) Option {
	return func(c *config) {
		otlp := ensureOTLP(c)
		otlp.endpoint = strings.TrimSpace(endpoint)
		otlp.exact = false
	}
}

// OTLPHeaders sets the headers sent with every OTLP request. The map is copied.
func OTLPHeaders(headers map[string]string) Option {
	return func(c *config) {
		otlp := ensureOTLP(c)
		if len(headers) == 0 {
			otlp.headers = nil
			return
		}
		copied := make(map[string]string, len(headers))
		for key, value := range headers {
			copied[key] = value
		}
		otlp.headers = copied
	}
}

// OTLPTimeout bounds one OTLP request, including its body read.
func OTLPTimeout(timeout time.Duration) Option {
	return func(c *config) {
		if timeout > 0 {
			ensureOTLP(c).timeout = timeout
		}
	}
}

// otlpSink batches spans and posts them to a collector.
type otlpSink struct {
	cfg         otlpConfig
	endpoint    string
	batchSize   int
	flushPeriod time.Duration
	client      *http.Client

	queue     chan record
	done      chan struct{}
	dropCount atomic.Uint64

	mu          sync.RWMutex
	closed      bool
	serviceName string
}

func newOTLPSink(otlp otlpConfig, cfg config, serviceName string) *otlpSink {
	if otlp.timeout <= 0 {
		otlp.timeout = defaultOTLPTimeout
	}
	if otlp.retryAttempts <= 0 {
		otlp.retryAttempts = defaultOTLPAttempts
	}
	if otlp.retryDelay <= 0 {
		otlp.retryDelay = defaultOTLPRetry
	}
	if cfg.batchSize <= 0 {
		cfg.batchSize = 512
	}
	if cfg.flushInterval <= 0 {
		cfg.flushInterval = 125 * time.Millisecond
	}
	if cfg.queueSize <= 0 {
		cfg.queueSize = 65536
	}
	sink := &otlpSink{
		cfg:         otlp,
		endpoint:    otlpEndpoint(otlp),
		batchSize:   cfg.batchSize,
		flushPeriod: cfg.flushInterval,
		client:      &http.Client{Timeout: otlp.timeout},
		queue:       make(chan record, cfg.queueSize),
		done:        make(chan struct{}),
		serviceName: serviceName,
	}
	go sink.run()
	return sink
}

func otlpEndpoint(cfg otlpConfig) string {
	if cfg.endpoint == "" {
		return defaultOTLPEndpoint
	}
	if cfg.exact {
		return cfg.endpoint
	}
	return strings.TrimRight(cfg.endpoint, "/") + "/v1/traces"
}

func (s *otlpSink) enqueue(rec record) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		s.dropCount.Add(1)
		return
	}
	select {
	case s.queue <- rec:
	default:
		s.dropCount.Add(1)
	}
}

func (s *otlpSink) dropped() uint64 {
	return s.dropCount.Load()
}

func (s *otlpSink) setServiceName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceName = name
}

func (s *otlpSink) currentServiceName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serviceName
}

func (s *otlpSink) close() error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	s.mu.Unlock()

	select {
	case <-s.done:
		return nil
	case <-time.After(drainTimeout):
		return errors.New("trace: timed out draining spans to OTLP")
	}
}

func (s *otlpSink) run() {
	defer close(s.done)

	ticker := time.NewTicker(s.flushPeriod)
	defer ticker.Stop()

	batch := make([]record, 0, s.batchSize)
	for {
		select {
		case rec, ok := <-s.queue:
			if !ok {
				s.flush(batch)
				return
			}
			batch = append(batch, rec)
			if len(batch) >= s.batchSize {
				s.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *otlpSink) flush(batch []record) {
	if len(batch) == 0 {
		return
	}
	payload, err := json.Marshal(buildOTLPPayload(batch, s.currentServiceName()))
	if err != nil {
		s.dropCount.Add(uint64(len(batch)))
		return
	}
	if err := s.post(payload); err != nil {
		s.dropCount.Add(uint64(len(batch)))
	}
}

func (s *otlpSink) post(payload []byte) error {
	var last error
	for attempt := 0; attempt < s.cfg.retryAttempts; attempt++ {
		err, retry := s.postOnce(payload)
		if err == nil {
			return nil
		}
		last = err
		if !retry {
			return err
		}
		if attempt+1 < s.cfg.retryAttempts {
			time.Sleep(s.cfg.retryDelay << attempt)
		}
	}
	return last
}

func (s *otlpSink) postOnce(payload []byte) (error, bool) {
	request, err := http.NewRequest(http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err, false
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range s.cfg.headers {
		request.Header.Set(key, value)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return err, true
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()

	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		return nil, false
	case retryableStatus(response.StatusCode):
		return fmt.Errorf("trace: OTLP collector returned %s", response.Status), true
	default:
		return fmt.Errorf("trace: OTLP collector returned %s", response.Status), false
	}
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
