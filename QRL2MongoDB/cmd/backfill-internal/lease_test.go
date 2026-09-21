package main

import (
	"QRL2MongoDB/configs"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMaintenanceLeaseEnsureHeldPinsOwnerAndGeneration(t *testing.T) {
	initial := configs.SyncerLease{
		Owner:      "maintenance-a",
		Generation: 7,
		ExpiresAt:  time.Now().Add(time.Minute),
	}
	var calls atomic.Int32
	keeper, err := startMaintenanceLease(
		initial,
		2*time.Minute,
		time.Hour,
		func(_ context.Context, lease configs.SyncerLease, ttl time.Duration) (configs.SyncerLease, error) {
			calls.Add(1)
			if lease.Owner != initial.Owner || lease.Generation != initial.Generation {
				t.Fatalf("renewed lease = %#v, want owner and generation from %#v", lease, initial)
			}
			if ttl != 2*time.Minute {
				t.Fatalf("renewal ttl = %s, want 2m", ttl)
			}
			lease.ExpiresAt = lease.ExpiresAt.Add(ttl)
			return lease, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.EnsureHeld(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("renew calls = %d, want 1", calls.Load())
	}
	if err := keeper.CloseAndRelease(context.Background(), func(_ context.Context, lease configs.SyncerLease) error {
		if lease.Owner != initial.Owner || lease.Generation != initial.Generation {
			t.Fatalf("released lease = %#v, want owner and generation from %#v", lease, initial)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceLeaseRejectsChangedGeneration(t *testing.T) {
	initial := configs.SyncerLease{Owner: "maintenance-a", Generation: 7}
	keeper, err := startMaintenanceLease(
		initial,
		2*time.Minute,
		time.Hour,
		func(_ context.Context, lease configs.SyncerLease, _ time.Duration) (configs.SyncerLease, error) {
			lease.Generation++
			return lease, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	err = keeper.EnsureHeld(context.Background())
	if err == nil {
		t.Fatal("expected changed generation to fail closed")
	}
	if keeper.Err() == nil {
		t.Fatal("lease loss was not retained")
	}
	_ = keeper.CloseAndRelease(context.Background(), nil)
}

func TestMaintenanceLeaseReleaseWaitsForRenewerToDrain(t *testing.T) {
	initial := configs.SyncerLease{Owner: "maintenance-a", Generation: 7}
	renewStarted := make(chan struct{})
	allowRenewalToFinish := make(chan struct{})
	var released atomic.Bool
	keeper, err := startMaintenanceLease(
		initial,
		2*time.Minute,
		time.Millisecond,
		func(_ context.Context, lease configs.SyncerLease, _ time.Duration) (configs.SyncerLease, error) {
			select {
			case <-renewStarted:
			default:
				close(renewStarted)
			}
			<-allowRenewalToFinish
			return configs.SyncerLease{}, errors.New("renewal failed")
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-renewStarted:
	case <-time.After(time.Second):
		t.Fatal("background renewal did not start")
	}

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- keeper.CloseAndRelease(context.Background(), func(context.Context, configs.SyncerLease) error {
			released.Store(true)
			return nil
		})
	}()

	time.Sleep(10 * time.Millisecond)
	if released.Load() {
		t.Fatal("lease released while renewal was still in flight")
	}
	close(allowRenewalToFinish)

	select {
	case closeErr := <-closeDone:
		if closeErr == nil {
			t.Fatal("expected renewal failure from close")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
	if !released.Load() {
		t.Fatal("lease was not released after renewal drained")
	}
}
