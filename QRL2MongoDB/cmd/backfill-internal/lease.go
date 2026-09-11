package main

import (
	"QRL2MongoDB/configs"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type renewLeaseFunc func(context.Context, configs.SyncerLease, time.Duration) (configs.SyncerLease, error)
type releaseLeaseFunc func(context.Context, configs.SyncerLease) error

// maintenanceLease keeps a one-off writer on the same owner and generation
// as the synchronizer. EnsureHeld performs a server-time renewal around each
// indexed-state mutation, while the background renewal covers long RPC reads.
type maintenanceLease struct {
	mu       sync.Mutex
	lease    configs.SyncerLease
	ttl      time.Duration
	renew    renewLeaseFunc
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	errMu    sync.RWMutex
	err      error
}

func startMaintenanceLease(
	initial configs.SyncerLease,
	ttl time.Duration,
	interval time.Duration,
	renew renewLeaseFunc,
) (*maintenanceLease, error) {
	if initial.Owner == "" || initial.Generation <= 0 || ttl < time.Millisecond || interval < time.Millisecond || renew == nil {
		return nil, fmt.Errorf("invalid maintenance lease configuration")
	}
	keeper := &maintenanceLease{
		lease:  initial,
		ttl:    ttl,
		renew:  renew,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go keeper.renewLoop(interval)
	return keeper, nil
}

func (keeper *maintenanceLease) EnsureHeld(ctx context.Context) error {
	if err := keeper.Err(); err != nil {
		return err
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	if err := keeper.Err(); err != nil {
		return err
	}
	renewed, err := keeper.renew(ctx, keeper.lease, keeper.ttl)
	if err != nil {
		lost := fmt.Errorf("renew chain-indexer maintenance lease: %w", err)
		keeper.recordError(lost)
		return lost
	}
	if renewed.Owner != keeper.lease.Owner || renewed.Generation != keeper.lease.Generation {
		lost := fmt.Errorf("renewed chain-indexer lease changed owner or generation")
		keeper.recordError(lost)
		return lost
	}
	keeper.lease = renewed
	return nil
}

func (keeper *maintenanceLease) Err() error {
	keeper.errMu.RLock()
	defer keeper.errMu.RUnlock()
	return keeper.err
}

func (keeper *maintenanceLease) CloseAndRelease(ctx context.Context, release releaseLeaseFunc) error {
	keeper.stopOnce.Do(func() { close(keeper.stopCh) })
	<-keeper.doneCh

	keeper.mu.Lock()
	lease := keeper.lease
	keeper.mu.Unlock()

	var releaseErr error
	if release != nil {
		releaseErr = release(ctx, lease)
	}
	return errors.Join(keeper.Err(), releaseErr)
}

func (keeper *maintenanceLease) renewLoop(interval time.Duration) {
	defer close(keeper.doneCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-keeper.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := keeper.EnsureHeld(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (keeper *maintenanceLease) recordError(err error) {
	keeper.errMu.Lock()
	if keeper.err == nil {
		keeper.err = err
	}
	keeper.errMu.Unlock()
}
