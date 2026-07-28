package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetOrComputeCachesUntilExpiry(t *testing.T) {
	c := New()
	var calls int32
	fn := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "v1", nil
	}

	v, err := c.GetOrCompute("k", 80*time.Millisecond, fn)
	if err != nil || v != "v1" {
		t.Fatalf("first call = (%v, %v), want (v1, nil)", v, err)
	}
	v, err = c.GetOrCompute("k", 80*time.Millisecond, fn)
	if err != nil || v != "v1" {
		t.Fatalf("second call = (%v, %v), want (v1, nil)", v, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("fn called %d times within TTL, want 1", got)
	}

	time.Sleep(100 * time.Millisecond)
	if _, err := c.GetOrCompute("k", 80*time.Millisecond, fn); err != nil {
		t.Fatalf("post-expiry call errored: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("fn called %d times after expiry, want 2", got)
	}
}

func TestGetOrComputeErrorIsNotCached(t *testing.T) {
	c := New()
	var calls int32
	boom := errors.New("boom")

	if _, err := c.GetOrCompute("k", time.Second, func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return nil, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	// The failed compute must not have been stored: the next call runs fn.
	v, err := c.GetOrCompute("k", time.Second, func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	})
	if err != nil || v != "ok" {
		t.Fatalf("retry = (%v, %v), want (ok, nil)", v, err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("fn called %d times, want 2", got)
	}
}

func TestGetOrComputeSingleflight(t *testing.T) {
	c := New()
	var calls int32
	const workers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, err := c.GetOrCompute("k", time.Second, func() (interface{}, error) {
				atomic.AddInt32(&calls, 1)
				// Hold the flight open so concurrent callers pile onto it
				// instead of racing past a finished compute.
				time.Sleep(30 * time.Millisecond)
				return "shared", nil
			})
			if err != nil || v != "shared" {
				t.Errorf("worker got (%v, %v), want (shared, nil)", v, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("fn called %d times across %d concurrent callers, want 1", got, workers)
	}
}

func TestJanitorEvictsAndStops(t *testing.T) {
	c := New()
	stop := c.StartJanitor(10 * time.Millisecond)

	if _, err := c.GetOrCompute("k", 5*time.Millisecond, func() (interface{}, error) {
		return "v", nil
	}); err != nil {
		t.Fatalf("seed compute errored: %v", err)
	}

	// Poll until the janitor has swept the expired entry (bounded wait so
	// a slow CI box doesn't flake).
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.RLock()
		_, present := c.store["k"]
		c.mu.RUnlock()
		if !present {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("janitor did not evict the expired entry within 2s")
		}
		time.Sleep(10 * time.Millisecond)
	}

	stop()
	stop() // stopping twice must be safe

	// A tick that raced the stop may have one eviction pass already
	// committed; give it time to drain before seeding the sentinel.
	time.Sleep(30 * time.Millisecond)

	// After stop, expired entries stay put: only the janitor removes them.
	c.mu.Lock()
	c.store["stale"] = entry{val: "old", expires: time.Now().Add(-time.Hour)}
	c.mu.Unlock()

	time.Sleep(50 * time.Millisecond) // several would-be ticks

	c.mu.RLock()
	_, still := c.store["stale"]
	c.mu.RUnlock()
	if !still {
		t.Fatal("entry evicted after stop: janitor goroutine kept running")
	}
}
