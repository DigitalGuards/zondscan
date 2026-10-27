package synchroniser

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"QRL2MongoDB/models"
)

type timingSourceFunc func(context.Context) (models.ChainTiming, error)

func (f timingSourceFunc) ChainTiming(ctx context.Context) (models.ChainTiming, error) { return f(ctx) }

type testTimingStore struct {
	timing       models.ChainTiming
	exists       bool
	genesis      int64
	cursorExists bool
	saves        int
	saveErr      error
}

func (s *testTimingStore) Load(context.Context) (models.ChainTiming, bool, error) {
	return s.timing, s.exists, nil
}
func (s *testTimingStore) CursorGenesis(context.Context) (int64, bool, error) {
	return s.genesis, s.cursorExists, nil
}
func (s *testTimingStore) Save(ctx context.Context, timing models.ChainTiming) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.saveErr != nil {
		return s.saveErr
	}
	s.timing, s.exists = timing, true
	s.saves++
	return nil
}

func timingTestLease(t *testing.T) {
	t.Helper()
	previous := chainMutationLeaseExpiry.Load()
	ConfigureChainMutationLease(time.Now().Add(time.Minute))
	t.Cleanup(func() { chainMutationLeaseExpiry.Store(previous) })
}

func TestInitializeChainTimingBeaconStates(t *testing.T) {
	timingTestLease(t)
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1700000000}
	for _, tc := range []struct {
		name     string
		stored   bool
		offline  bool
		changed  string
		cursor   int64
		conflict bool
		known    bool
	}{
		{name: "reachable fresh", known: true},
		{name: "reachable stored matching", stored: true, cursor: timing.GenesisTime, known: true},
		{name: "offline stored", offline: true, stored: true, cursor: timing.GenesisTime, known: true},
		{name: "offline fresh", offline: true},
		{name: "offline cursor only", offline: true, cursor: timing.GenesisTime},
		{name: "different slots", stored: true, changed: "slots", conflict: true},
		{name: "different seconds", stored: true, changed: "seconds", conflict: true},
		{name: "different genesis", stored: true, changed: "genesis", conflict: true},
		{name: "cursor disagrees with beacon", cursor: timing.GenesisTime + 1, conflict: true},
		{name: "cursor disagrees with cached clock", stored: true, offline: true, cursor: timing.GenesisTime + 1, conflict: true},
		{name: "invalid stored clock", stored: true, offline: true, changed: "invalid", conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &testTimingStore{timing: timing, exists: tc.stored, genesis: tc.cursor, cursorExists: tc.cursor != 0}
			switch tc.changed {
			case "slots":
				store.timing.SlotsPerEpoch *= 2
			case "seconds":
				store.timing.SecondsPerSlot *= 2
			case "genesis":
				store.timing.GenesisTime++
			case "invalid":
				store.timing.SecondsPerSlot = 0
			}
			source := timingSourceFunc(func(context.Context) (models.ChainTiming, error) {
				if tc.offline {
					return models.ChainTiming{}, errors.New("beacon unavailable")
				}
				return timing, nil
			})
			state := newChainTimingState()
			err := state.initialize(context.Background(), source, store)
			if tc.conflict {
				if !errors.Is(err, errChainTimingConflict) {
					t.Fatalf("expected chain conflict, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			ready := false
			select {
			case <-state.ready:
				ready = true
			default:
			}
			if ready != tc.known {
				t.Fatalf("ready=%v, want %v", ready, tc.known)
			}
			if ready && state.timing != timing {
				t.Fatalf("timing=%+v", state.timing)
			}
			wantSaves := 0
			if !tc.offline && !tc.conflict {
				wantSaves = 1
			}
			if store.saves != wantSaves {
				t.Fatalf("saves=%d, want %d", store.saves, wantSaves)
			}
		})
	}
}

func TestChainTimingRecoveryWakesWaitingWorkers(t *testing.T) {
	timingTestLease(t)
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1700000000}
	store := &testTimingStore{}
	var online atomic.Bool
	source := timingSourceFunc(func(context.Context) (models.ChainTiming, error) {
		if !online.Load() {
			return models.ChainTiming{}, errors.New("beacon unavailable")
		}
		return timing, nil
	})
	state := newChainTimingState()
	if err := state.initialize(context.Background(), source, store); err != nil {
		t.Fatal(err)
	}
	// Successful initialization lets main start execution indexing immediately.
	stopCh := make(chan struct{})
	ran := make(chan string, 3)
	for _, name := range []string{"Validator history", "Validator snapshot", "Slot assignments"} {
		state.startWorker(stopCh, name, func(ctx context.Context) (bool, error) {
			return false, slotAssignmentMutation(ctx, func(context.Context) error {
				if state.timing != timing || state.beacon == nil {
					t.Error("worker started without timing")
				}
				ran <- name
				return nil
			})
		})
	}
	t.Cleanup(func() {
		close(stopCh)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := WaitForBackgroundWorkers(ctx); err != nil {
			t.Error(err)
		}
	})
	select {
	case name := <-ran:
		t.Fatalf("%s ran before discovery", name)
	case <-time.After(20 * time.Millisecond):
	}
	online.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := state.retry(ctx, time.Millisecond, source, store); err != nil {
		t.Fatal(err)
	}
	if !store.exists || store.timing != timing || store.saves != 1 {
		t.Fatalf("store=%+v", store)
	}
	for range 3 {
		select {
		case <-ran:
		case <-ctx.Done():
			t.Fatal("worker did not start promptly after recovery")
		}
	}
}

func TestChainTimingRetryRejectsConflicts(t *testing.T) {
	timingTestLease(t)
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1700000000}
	for _, cached := range []bool{false, true} {
		store := &testTimingStore{timing: timing, exists: cached, cursorExists: true, genesis: timing.GenesisTime}
		state := newChainTimingState()
		offline := timingSourceFunc(func(context.Context) (models.ChainTiming, error) { return models.ChainTiming{}, errors.New("offline") })
		if err := state.initialize(context.Background(), offline, store); err != nil {
			t.Fatal(err)
		}
		source := timingSourceFunc(func(context.Context) (models.ChainTiming, error) {
			changed := timing
			changed.GenesisTime++
			return changed, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := state.retry(ctx, time.Millisecond, source, store)
		cancel()
		if !errors.Is(err, errChainTimingConflict) || store.saves != 0 {
			t.Fatalf("err=%v, saves=%d", err, store.saves)
		}
	}
}

func TestChainTimingDiscoveryRequiresLeaseAndPersistence(t *testing.T) {
	timingTestLease(t)
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1700000000}
	source := timingSourceFunc(func(context.Context) (models.ChainTiming, error) { return timing, nil })
	for _, tc := range []struct {
		name    string
		expiry  time.Time
		saveErr error
	}{
		{name: "expired lease", expiry: time.Now().Add(-time.Second)},
		{name: "persistence failure", expiry: time.Now().Add(time.Minute), saveErr: errors.New("write failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ConfigureChainMutationLease(tc.expiry)
			store := &testTimingStore{saveErr: tc.saveErr}
			state := newChainTimingState()
			if err := state.initialize(context.Background(), source, store); err == nil {
				t.Fatal("expected error")
			}
			select {
			case <-state.ready:
				t.Fatal("published unpersisted timing")
			default:
			}
			if store.saves != 0 {
				t.Fatal("unleased or failed write saved timing")
			}
		})
	}
}

func TestChainTimingRetryAndWaitStopOnShutdown(t *testing.T) {
	timingTestLease(t)
	for _, inFlight := range []bool{false, true} {
		stopCh := make(chan struct{})
		ctx, cancel := beaconWorkerContext(stopCh)
		store := &testTimingStore{}
		state := newChainTimingState()
		started := make(chan struct{})
		source := timingSourceFunc(func(ctx context.Context) (models.ChainTiming, error) {
			close(started)
			<-ctx.Done()
			return models.ChainTiming{}, ctx.Err()
		})
		done := make(chan error, 1)
		interval := time.Hour
		if inFlight {
			interval = time.Millisecond
		}
		go func() { done <- state.retry(ctx, interval, source, store) }()
		state.startWorker(stopCh, "waiting", func(context.Context) (bool, error) { t.Error("waiting worker ran"); return false, nil })
		if inFlight {
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("retry did not start")
			}
		}
		close(stopCh)
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Fatal("retry ignored shutdown")
		}
		cancel()
		drain, drainCancel := context.WithTimeout(context.Background(), time.Second)
		if err := WaitForBackgroundWorkers(drain); err != nil {
			t.Error(err)
		}
		drainCancel()
		if store.saves != 0 {
			t.Fatal("shutdown persisted timing")
		}
	}
}
