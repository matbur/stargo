package session

import (
	"context"
	"sync"
	"time"
)

// Record is the stored state of one session.
//
// Created anchors the absolute lifetime. Expires is the earlier of the idle
// deadline and the absolute deadline, so a store can drop expired records with
// a single timestamp.
type Record struct {
	// Values is the session data. Durable stores serialize it, so applications
	// should keep values JSON-friendly.
	Values map[string]any
	// Created is when the session was first stored.
	Created time.Time
	// Expires is when the record stops being valid.
	Expires time.Time
}

// Store keeps session records by id.
//
// A store may serialize Values. Load reports false for an unknown or expired
// id. Implementations must be safe for concurrent use.
type Store interface {
	Load(ctx context.Context, id string) (Record, bool, error)
	Save(ctx context.Context, id string, record Record) error
	Delete(ctx context.Context, id string) error
}

// MemoryStore keeps session records in memory.
//
// Expired records are removed on Load and Save. It starts no background
// goroutine; call Sweep periodically in a long-running process that creates
// many short-lived sessions.
type MemoryStore struct {
	mu      sync.Mutex
	records map[string]Record
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]Record)}
}

// Load returns the record for id, or false when it is unknown or expired.
func (s *MemoryStore) Load(ctx context.Context, id string) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok {
		return Record{}, false, nil
	}
	if !record.Expires.After(time.Now()) {
		delete(s.records, id)
		return Record{}, false, nil
	}
	return cloneRecord(record), true, nil
}

// Save writes a record, replacing any previous record for the id.
func (s *MemoryStore) Save(ctx context.Context, id string, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[id] = cloneRecord(record)
	return nil
}

// Delete removes the record for id. Deleting an unknown id is not an error.
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, id)
	return nil
}

// Sweep removes expired records and returns how many it removed.
func (s *MemoryStore) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, record := range s.records {
		if !record.Expires.After(now) {
			delete(s.records, id)
			removed++
		}
	}
	return removed
}

// Len returns the number of stored records, including expired records that
// have not been swept yet.
func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func cloneRecord(record Record) Record {
	record.Values = cloneValues(record.Values)
	return record
}

func cloneValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
