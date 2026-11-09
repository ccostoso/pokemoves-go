package pokeapi

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

type Cache[V any] struct {
	mu      sync.Mutex
	entries map[string]entry[V]
	group   singleflight.Group
	ttl     time.Duration
	max     int
	now     func() time.Time
}

func NewCache[V any](ttl time.Duration, max int) *Cache[V] {
	if max <= 0 {
		panic("max must be greater than 0")
	}

	return &Cache[V]{
		entries: make(map[string]entry[V]),
		ttl:     ttl,
		max:     max,
		now:     time.Now,
	}
}

func (c *Cache[V]) Get(ctx context.Context, key string, load func(context.Context) (V, error)) (V, error) {
	// Fresh hit (value exists and is not expired)
	if v, ok := c.fresh(key); ok {
		return v, nil
	}

	// Cache miss (value does not exist or is expired)
	// Use singleflight.Group to ensure only one goroutine loads the value for the given key at a time.
	// Do() ensures that only one goroutine loads the value for the given key at a time.
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Check if another goroutine has already loaded the value from the cache.
		if v, ok := c.fresh(key); ok {
			return v, nil // Return the fresh value from the cache.
		}

		// Load the value using the provided load function.
		// Use context.WithoutCancel to avoid cancellation propagation to other goroutines.
		val, err := load(context.WithoutCancel(ctx))
		if err != nil {
			// Return stale value if available
			if stale, ok := c.stale(key); ok {
				return stale, nil
			}
			var zero V
			return zero, err
		}

		// At this point, the value has been successfully loaded from the source and can be stored in the cache.
		// Store the loaded value in the cache with an expiration time.
		c.store(key, val)

		// Return the loaded value.
		return val, nil
	})

	// If an error occurred and no stale value was returned, propagate the error.
	if err != nil {
		var zero V
		return zero, err
	}
	return v.(V), nil
}

// fresh returns the cached value for the given key if it exists and is not expired.
func (c *Cache[V]) fresh(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if ok && e.expires.After(c.now()) {
		return e.value, true
	}
	var zero V
	return zero, false
}

// stale returns the cached value for the given key regardless of its expiration status.
func (c *Cache[V]) stale(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e.value, ok
}

// store saves val under key, evicting entries first if the cache is full.
func (c *Cache[V]) store(key string, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		c.evictLocked(now)
	}
	c.entries[key] = entry[V]{value: val, expires: now.Add(c.ttl)}
}

// evictLocked frees at least one slot. The caller must hold c.mu.
func (c *Cache[V]) evictLocked(now time.Time) {
	// Delete every expired entry
	for k, e := range c.entries {
		if !e.expires.After(now) {
			delete(c.entries, k)
		}
	}

	// If the map is still full, delete the entry with the earliest expires
	if len(c.entries) >= c.max {
		var earliestKey string
		// Initialize earliestExpires to the zero value of time.Time and first to true to track the earliest expiration time.
		var earliestExpires time.Time
		first := true
		for k, e := range c.entries {
			// Keep track of the entry with the earliest expiration time to evict it if needed.
			// We check first to see if this is the first entry we're examining, or if it has an earlier expiration than the current earliest.
			if first || !e.expires.After(earliestExpires) {
				earliestKey = k
				earliestExpires = e.expires
				first = false
			}
		}
		delete(c.entries, earliestKey)
	}
}
