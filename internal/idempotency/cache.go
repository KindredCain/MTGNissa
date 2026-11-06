// Package idempotency coordinates bounded, process-local mutation deduplication.
package idempotency

import (
	"context"
	"sync"
	"time"

	"mtgnissa/internal/apperror"
)

// Cache retains successful mutation fingerprints for thirty minutes.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]*entry
	capacity int
}

type entry struct {
	fingerprint string
	done        chan struct{}
	expires     time.Time
	err         error
}

// New creates a cache with a fixed maximum number of pending and completed intents.
func New(capacity int) *Cache {
	return &Cache{entries: make(map[string]*entry), capacity: capacity}
}

// Do executes a mutation once, waits for duplicates, and reports whether it was deduplicated.
func (c *Cache) Do(ctx context.Context, key, fingerprint string, mutation func() error) (bool, error) {
	c.mu.Lock()
	now := time.Now()
	for k, e := range c.entries {
		if !e.expires.IsZero() && !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	if existing := c.entries[key]; existing != nil {
		c.mu.Unlock()
		if existing.fingerprint != fingerprint {
			return false, apperror.New(apperror.CodeIdempotencyConflict, "idempotency key was already used for a different request")
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-existing.done:
			return existing.err == nil, existing.err
		}
	}
	if len(c.entries) >= c.capacity {
		c.mu.Unlock()
		return false, apperror.New(apperror.CodeDependencyUnavailable, "mutation cache is temporarily full")
	}
	e := &entry{fingerprint: fingerprint, done: make(chan struct{})}
	c.entries[key] = e
	c.mu.Unlock()
	return false, c.execute(key, e, mutation)
}

func (c *Cache) execute(key string, e *entry, mutation func() error) error {
	err := error(apperror.New(apperror.CodeInternalError, "mutation did not complete"))
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		e.err = err
		if err == nil {
			e.expires = time.Now().Add(30 * time.Minute)
		} else {
			delete(c.entries, key)
		}
		close(e.done)
	}()
	err = mutation()
	return err
}
