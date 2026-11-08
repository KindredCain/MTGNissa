package idempotency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mtgnissa/internal/apperror"
)

func TestConcurrentRequestsMutateOnceAndRejectDifferentIntent(t *testing.T) {
	cache := New(4)
	var calls atomic.Int32
	var originals atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			duplicate, err := cache.Do(context.Background(), "key", "same", func() error {
				calls.Add(1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
			if !duplicate {
				originals.Add(1)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || originals.Load() != 1 {
		t.Fatalf("mutations=%d originals=%d", calls.Load(), originals.Load())
	}
	_, err := cache.Do(context.Background(), "key", "different", func() error {
		t.Fatal("conflicting request executed")
		return nil
	})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeIdempotencyConflict {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestFailuresRetryAndCapacityPreservesUnexpiredEntries(t *testing.T) {
	cache := New(1)
	failure := errors.New("failed")
	_, err := cache.Do(context.Background(), "key", "intent", func() error {
		return failure
	})
	if err != failure || len(cache.entries) != 0 {
		t.Fatal("failed mutation retained")
	}
	duplicate, err := cache.Do(context.Background(), "key", "intent", func() error {
		return nil
	})
	if duplicate || err != nil {
		t.Fatalf("retry duplicate=%v error=%v", duplicate, err)
	}
	_, err = cache.Do(context.Background(), "other", "intent", func() error {
		t.Fatal("full cache executed mutation")
		return nil
	})
	if err == nil || len(cache.entries) != 1 {
		t.Fatal("capacity failed to preserve completed record")
	}
	cache.entries["key"].expires = time.Now().Add(-time.Second)
	_, err = cache.Do(context.Background(), "other", "intent", func() error {
		return nil
	})
	if err != nil || len(cache.entries) != 1 {
		t.Fatal("expired record was not reclaimed")
	}
}

func TestCanceledWaitAndPanickedOwnerRelease(t *testing.T) {
	cache := New(4)
	cache.entries["pending"] = &entry{fingerprint: "intent", done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.Do(ctx, "pending", "intent", func() error {
		t.Fatal("waiter executed mutation")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected mutation panic")
			}
		}()
		_, _ = cache.Do(context.Background(), "panic", "intent", func() error {
			panic("failure")
		})
	}()
	if cache.entries["panic"] != nil {
		t.Fatal("panic left a pending entry")
	}
}

func TestResourceIDSurvivesRetriesAndFailedMutationsDoNotComplete(t *testing.T) {
	cache := New(4)
	calls := 0
	mutation := func() (uint64, error) {
		calls++
		return 42, nil
	}
	id, duplicate, err := cache.DoResource(context.Background(), "create", "intent", mutation)
	if err != nil || duplicate || id != 42 {
		t.Fatalf("first creation: id=%d duplicate=%v error=%v", id, duplicate, err)
	}
	id, duplicate, err = cache.DoResource(context.Background(), "create", "intent", mutation)
	if err != nil || !duplicate || id != 42 || calls != 1 {
		t.Fatalf("creation replay: id=%d duplicate=%v calls=%d error=%v", id, duplicate, calls, err)
	}
	failure := errors.New("transaction failed")
	if _, _, err := cache.DoResource(context.Background(), "copy", "intent", func() (uint64, error) {
		return 99, failure
	}); err != failure {
		t.Fatal(err)
	}
	id, duplicate, err = cache.DoResource(context.Background(), "copy", "intent", mutation)
	if err != nil || duplicate || id != 42 || calls != 2 {
		t.Fatalf("failed mutation retry: id=%d duplicate=%v error=%v", id, duplicate, err)
	}
}
