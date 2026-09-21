package synchroniser

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPendingTokenQueueWorkerRetriesOnQuietChainAndDrains(t *testing.T) {
	stopCh := make(chan struct{})
	var calls atomic.Int32
	called := make(chan struct{}, 4)
	process := func() error {
		count := calls.Add(1)
		select {
		case called <- struct{}{}:
		default:
		}
		if count == 1 {
			return errors.New("retryable receipt failure")
		}
		return nil
	}

	if !startPendingTokenQueueJob(stopCh, 5*time.Millisecond, process) {
		t.Fatal("queue worker was not registered")
	}
	for index := 0; index < 2; index++ {
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatalf("queue pass %d did not run", index+1)
		}
	}
	close(stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := WaitForBackgroundWorkers(ctx); err != nil {
		t.Fatalf("registered queue worker did not drain: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("queue calls = %d, want a quiet-chain retry", calls.Load())
	}
}
