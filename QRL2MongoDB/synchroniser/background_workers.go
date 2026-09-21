package synchroniser

import (
	"context"
	"sync"
)

// workerRegistry tracks every auxiliary goroutine that can write indexed
// state. Shutdown closes the shared stop channel, seals this registry against
// late starts, and waits for all registered writers before releasing the
// cross-process syncer lease.
type workerRegistry struct {
	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
}

var backgroundWorkers workerRegistry

func startBackgroundWorker(worker func()) bool {
	backgroundWorkers.mu.Lock()
	if backgroundWorkers.closing {
		backgroundWorkers.mu.Unlock()
		return false
	}
	backgroundWorkers.wg.Add(1)
	backgroundWorkers.mu.Unlock()

	go func() {
		defer backgroundWorkers.wg.Done()
		worker()
	}()
	return true
}

// BeginBackgroundShutdown prevents auxiliary services from starting after a
// shutdown signal. It must be called immediately after the shared stop channel
// is closed and before waiting for Sync to drain.
func BeginBackgroundShutdown() {
	backgroundWorkers.mu.Lock()
	backgroundWorkers.closing = true
	backgroundWorkers.mu.Unlock()
}

// WaitForBackgroundWorkers waits until every registered auxiliary writer has
// returned, or until ctx expires.
func WaitForBackgroundWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		backgroundWorkers.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
