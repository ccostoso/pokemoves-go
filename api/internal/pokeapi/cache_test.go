package pokeapi

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestCacheHitSkipsLoader ensures that a cache hit does not invoke the loader function.
func TestCacheHitSkipsLoader(t *testing.T) {
	// Create a new cache with a ttl of a minute and a max of 10 entries
	c := NewCache[string](time.Minute, 10)
	loaderCalls := 0
	load := func(ctx context.Context) (string, error) {
		loaderCalls++
		return "value", nil
	}

	// First Get should call the loader
	v, err := c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	// Second Get should hit the cache and not call the loader again
	v, err = c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	if loaderCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loaderCalls)
	}
}

// TestCacheExpiry ensures that the cache correctly expires entries after the TTL has passed.
func TestCacheExpiry(t *testing.T) {
	now := time.Now()
	c := NewCache[string](time.Minute, 10)
	c.now = func() time.Time { return now }

	loaderCalls := 0
	load := func(ctx context.Context) (string, error) {
		loaderCalls++
		return "value", nil
	}

	// First Get should call the loader
	v, err := c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	// Advance the fake clock past the TTL
	now = now.Add(time.Minute + time.Second)

	// Second Get should call the loader again due to expiry
	v, err = c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	if loaderCalls != 2 {
		t.Fatalf("expected loader to be called twice, got %d", loaderCalls)
	}
}

// TestCacheStaleOnError ensures that if the loader fails after the cache entry has expired, the stale value is returned with a nil error.
func TestCacheStaleOnError(t *testing.T) {
	c := NewCache[string](time.Minute, 10)

	loaderCalls := 0
	load := func(ctx context.Context) (string, error) {
		loaderCalls++
		if loaderCalls == 1 {
			return "value", nil
		}
		return "", context.DeadlineExceeded
	}

	// First call should succeed
	v, err := c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	// Expire cache entry by advancing fake clock
	c.now = func() time.Time { return time.Now().Add(time.Minute + time.Second) }

	// Second call should return stale value with nil error
	v, err = c.Get(context.Background(), "key", load)
	if err != nil || v != "value" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	if loaderCalls != 2 {
		t.Fatalf("expected loader to be called twice, got %d", loaderCalls)
	}
}

// TestCacheErrorWithoutStale ensures that if the loader fails and there is no stale value, the error is propagated.
func TestCacheErrorWithoutStale(t *testing.T) {
	c := NewCache[string](time.Minute, 10)

	loaderCalls := 0
	load := func(ctx context.Context) (string, error) {
		loaderCalls++
		return "", context.DeadlineExceeded
	}

	// First call should fail
	v, err := c.Get(context.Background(), "key", load)
	if err == nil || v != "" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	if loaderCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loaderCalls)
	}
}

// TestCacheErrorNotCached ensures that if the loader fails, the cache does not store the error result.
func TestCacheErrorNotCached(t *testing.T) {
	c := NewCache[string](time.Minute, 10)

	loaderCalls := 0
	load := func(ctx context.Context) (string, error) {
		loaderCalls++
		return "", context.DeadlineExceeded
	}

	// First call should fail
	v, err := c.Get(context.Background(), "key", load)
	if err == nil || v != "" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	// Second call should also fail and not return a cached value
	v, err = c.Get(context.Background(), "key", load)
	if err == nil || v != "" {
		t.Fatalf("unexpected result: %v, %v", v, err)
	}

	if loaderCalls != 2 {
		t.Fatalf("expected loader to be called twice, got %d", loaderCalls)
	}
}

// TestCacheSingleflight ensures that concurrent requests for the same key only invoke the loader once.
func TestCacheSingleflight(t *testing.T) {
	c := NewCache[string](time.Minute, 10)

	var loaderCalls atomic.Int32
	load := func(ctx context.Context) (string, error) {
		loaderCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "value", nil
	}

	const goroutines = 20
	results := make(chan string, goroutines)
	errors := make(chan error, goroutines)

	// Launch multiple goroutines to concurrently request the same key.
	for i := 0; i < goroutines; i++ {
		go func() {
			v, err := c.Get(context.Background(), "key", load)
			results <- v
			errors <- err
		}()
	}

	// Wait for all goroutines to complete and collect their results.
	for i := 0; i < goroutines; i++ {
		v := <-results
		err := <-errors
		if err != nil || v != "value" {
			t.Fatalf("unexpected result: %v, %v", v, err)
		}
	}

	if loaderCalls.Load() != 1 {
		t.Fatalf("expected loader to be called once, got %d", loaderCalls.Load())
	}
}

// TestCacheEviction ensures that the cache evicts the least recently used entry when the maximum size is exceeded.
func TestCacheEviction(t *testing.T) {
	now := time.Now()
	c := NewCache[string](time.Hour, 2)
	c.now = func() time.Time { return now }
	load := func(ctx context.Context) (string, error) {
		return "value", nil
	}

	// Add three keys
	for _, key := range []string{"key1", "key2", "key3"} {
		if _, err := c.Get(context.Background(), key, load); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		now = now.Add(time.Second)
	}

	if len(c.entries) != 2 {
		t.Fatalf("expected cache size to be 2, got %d", len(c.entries))
	}

	// Expect key1 to be evicted as it has the earliest expiry
	if _, ok := c.entries["key1"]; ok {
		t.Fatalf("expected key1 (earliest expiry) to be evicted")
	}

	// Ensure that the remaining keys are present in the cache
	for _, key := range []string{"key2", "key3"} {
		if _, ok := c.entries[key]; !ok {
			t.Fatalf("expected key %s to be present", key)
		}
	}
}

// TestCacheEvictionClearsAllExpired ensures that when all existing entries are expired, adding a new entry clears them from the cache.
func TestCacheEvictionClearsAllExpired(t *testing.T) {
	now := time.Now()
	c := NewCache[string](time.Minute, 3)
	c.now = func() time.Time { return now }
	load := func(ctx context.Context) (string, error) { return "value", nil }

	for _, key := range []string{"a", "b", "c"} {
		c.Get(context.Background(), key, load)
	}

	// Expire all three existing keys by advancing the time beyond their TTL
	now = now.Add(2 * time.Minute)

	// Add a new key to trigger eviction of expired entries
	c.Get(context.Background(), "d", load)

	if len(c.entries) != 1 {
		t.Fatalf("expected only d to remain, got %d entries", len(c.entries))
	}

	// Ensure that the new key "d" is present in the cache.
	if _, ok := c.entries["d"]; !ok {
		t.Fatalf("expected key d to be present")
	}
}
