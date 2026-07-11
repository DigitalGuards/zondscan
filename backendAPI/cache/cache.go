// Package cache provides a tiny in-process TTL cache with singleflight
// deduplication, used to absorb concurrent traffic on hot read endpoints
// (/overview, /blocks, /txs, /pending-transactions, /latestblock,
// /address/aggregate).
//
// Without this layer, every page-view fans out into 1-8 MongoDB queries,
// and the homepage polls every 30 s, so at modest concurrency the same
// queries are run hundreds of times per second. With the cache, identical
// requests within the TTL window are served from memory, and the
// singleflight Group ensures only ONE goroutine recomputes a key when it
// expires (no "thundering herd" against MongoDB).
//
// Bounded by design: only a small set of route+params keys exist, so we
// don't need LRU eviction. A periodic janitor purges expired entries to
// keep the map from leaking memory under unusual key churn.
package cache

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry struct {
	val     interface{}
	expires time.Time
}

// TTLCache is concurrent-safe. Zero value is not usable, call New().
type TTLCache struct {
	mu    sync.RWMutex
	store map[string]entry
	sf    singleflight.Group
}

func New() *TTLCache {
	return &TTLCache{store: make(map[string]entry)}
}

// GetOrCompute returns the cached value for key if it's still fresh.
// Otherwise it calls fn (deduped across concurrent callers for the same
// key via singleflight), caches the result with the given TTL, and
// returns it. If fn returns an error nothing is cached.
func (c *TTLCache) GetOrCompute(key string, ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.get(key); ok {
		return v, nil
	}
	v, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Re-check under singleflight in case another caller just populated
		// the entry while we were queued.
		if v, ok := c.get(key); ok {
			return v, nil
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.store[key] = entry{val: val, expires: time.Now().Add(ttl)}
		c.mu.Unlock()
		return val, nil
	})
	return v, err
}

func (c *TTLCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	e, ok := c.store[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.val, true
}

// StartJanitor spawns a goroutine that walks the store every `interval`
// and deletes expired entries. Returns immediately; the returned stop
// function terminates the goroutine (safe to call more than once) so a
// shutdown hook can reclaim it.
func (c *TTLCache) StartJanitor(interval time.Duration) func() {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			// Check done first so a tick that raced the stop call can't
			// keep winning the select below; at most one already-committed
			// eviction pass runs after stop returns.
			select {
			case <-done:
				return
			default:
			}
			select {
			case <-t.C:
				c.evictExpired()
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
	}
}

// evictExpired uses a two-phase lock so a large cache doesn't stall reads
// while the janitor scans: a read lock to collect candidate keys, then a
// brief write lock to delete them (re-checking expiry to avoid a TOCTOU
// race where a concurrent GetOrCompute refreshed the entry between
// passes).
func (c *TTLCache) evictExpired() {
	now := time.Now()

	c.mu.RLock()
	var expiredKeys []string
	for k, e := range c.store {
		if now.After(e.expires) {
			expiredKeys = append(expiredKeys, k)
		}
	}
	c.mu.RUnlock()

	if len(expiredKeys) == 0 {
		return
	}

	c.mu.Lock()
	for _, k := range expiredKeys {
		if e, ok := c.store[k]; ok && now.After(e.expires) {
			delete(c.store, k)
		}
	}
	c.mu.Unlock()
}
