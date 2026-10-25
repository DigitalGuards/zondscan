package synchroniser

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
)

type testSlotSource struct {
	finalized         int64
	unavailableBefore int64
	failEpoch         *int64
	incomplete        bool
	requested         []int64
}

func (s *testSlotSource) GenesisTime(context.Context) (int64, error)    { return 1700000000, nil }
func (s *testSlotSource) FinalizedEpoch(context.Context) (int64, error) { return s.finalized, nil }
func (s *testSlotSource) Assignments(_ context.Context, epoch int64) ([]models.SlotAssignment, error) {
	s.requested = append(s.requested, epoch)
	if epoch < s.unavailableBefore {
		return nil, rpc.ErrEpochUnavailable
	}
	if s.failEpoch != nil && epoch == *s.failEpoch {
		return nil, errors.New("transient beacon failure")
	}
	rows := []models.SlotAssignment{}
	for slot := epoch * 8; slot < (epoch+1)*8; slot++ {
		if slot == 0 {
			continue
		}
		rows = append(rows, models.SlotAssignment{Epoch: epoch, Slot: slot, ValidatorIndex: "0", Pubkey: "0xaabb"})
	}
	if s.incomplete {
		rows = rows[:len(rows)-1]
	}
	return rows, nil
}

type testSlotStore struct {
	state         models.SlotAssignmentCursor
	exists        bool
	rows          map[int64]models.SlotAssignment
	failAfterRows bool
}

func (s *testSlotStore) Load(context.Context) (models.SlotAssignmentCursor, bool, error) {
	return s.state, s.exists, nil
}
func (s *testSlotStore) Save(ctx context.Context, state models.SlotAssignmentCursor, rows []models.SlotAssignment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.rows == nil {
		s.rows = make(map[int64]models.SlotAssignment)
	}
	for _, row := range rows {
		s.rows[row.Slot] = row
	}
	if s.failAfterRows && len(rows) > 0 {
		return errors.New("cursor write failed")
	}
	s.state, s.exists = state, true
	return nil
}

func TestSlotWalkerFindsEarliestHistoryAndKeepsUp(t *testing.T) {
	ctx := context.Background()
	source := &testSlotSource{finalized: 5, unavailableBefore: 2}
	store := &testSlotStore{}
	more, err := walkSlotAssignments(ctx, source, store)
	if err != nil || more {
		t.Fatalf("walk = %t, %v", more, err)
	}
	if store.state.NextEpoch != 6 || store.state.FirstAvailableEpoch == nil || *store.state.FirstAvailableEpoch != 2 || store.state.UnavailableEpochs != 2 || len(store.rows) != 32 {
		t.Fatalf("state = %+v, rows = %d", store.state, len(store.rows))
	}
	source.finalized = 7
	source.requested = nil
	if _, err := walkSlotAssignments(ctx, source, store); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(source.requested, []int64{6, 7}) {
		t.Fatalf("resumed epochs = %v", source.requested)
	}
	if _, exists := store.rows[8*8]; exists {
		t.Fatal("epoch above the finalized checkpoint was indexed")
	}
}

func TestSlotWalkerBoundsBatchAndRepresentsGenesis(t *testing.T) {
	source := &testSlotSource{finalized: 20}
	store := &testSlotStore{}
	more, err := walkSlotAssignments(context.Background(), source, store)
	if err != nil || !more || store.state.NextEpoch != slotAssignmentBatchSize {
		t.Fatalf("walk = %t, %v; state = %+v", more, err, store.state)
	}
	if row, ok := store.rows[0]; !ok || row.ValidatorIndex != "" || row.Pubkey != "" {
		t.Fatalf("genesis assignment = %+v, %t", row, ok)
	}
	if len(store.rows) != 8*slotAssignmentBatchSize {
		t.Fatalf("rows = %d", len(store.rows))
	}
	if _, err := walkSlotAssignments(context.Background(), source, store); err != nil {
		t.Fatal(err)
	}
	if store.state.NextEpoch != 21 {
		t.Fatalf("next epoch = %d", store.state.NextEpoch)
	}
}

func TestSlotWalkerRetriesTransientAndIncompleteEpochs(t *testing.T) {
	failed := int64(1)
	source := &testSlotSource{finalized: 3, failEpoch: &failed}
	store := &testSlotStore{}
	if _, err := walkSlotAssignments(context.Background(), source, store); err == nil {
		t.Fatal("expected transient failure")
	}
	if store.state.NextEpoch != 1 || len(store.rows) != 8 {
		t.Fatalf("state = %+v, rows = %d", store.state, len(store.rows))
	}
	source.failEpoch = nil
	source.incomplete = true
	if _, err := walkSlotAssignments(context.Background(), source, store); err == nil {
		t.Fatal("expected incomplete duty failure")
	}
	if store.state.NextEpoch != 1 {
		t.Fatal("incomplete epoch advanced cursor")
	}
	source.incomplete = false
	if _, err := walkSlotAssignments(context.Background(), source, store); err != nil {
		t.Fatal(err)
	}
	if store.state.NextEpoch != 4 {
		t.Fatal("retry did not finish")
	}
}

func TestSlotWalkerReplaysRowsAfterCursorWriteFailure(t *testing.T) {
	source := &testSlotSource{finalized: 2}
	store := &testSlotStore{failAfterRows: true}
	if _, err := walkSlotAssignments(context.Background(), source, store); err == nil {
		t.Fatal("expected cursor failure")
	}
	if store.state.NextEpoch != 0 || len(store.rows) != 8 {
		t.Fatalf("partial state = %+v, rows = %d", store.state, len(store.rows))
	}
	store.failAfterRows = false
	if _, err := walkSlotAssignments(context.Background(), source, store); err != nil {
		t.Fatal(err)
	}
	if store.state.NextEpoch != 3 || len(store.rows) != 24 {
		t.Fatal("idempotent replay failed")
	}
}

func TestSlotAssignmentWritesRequireLeaseAndLiveContext(t *testing.T) {
	previous := chainMutationLeaseExpiry.Load()
	t.Cleanup(func() { chainMutationLeaseExpiry.Store(previous) })
	for _, expiry := range []int64{0, time.Now().Add(-time.Second).UnixNano()} {
		chainMutationLeaseExpiry.Store(expiry)
		called := false
		err := slotAssignmentMutation(context.Background(), func(context.Context) error { called = true; return nil })
		if err == nil || called {
			t.Fatalf("unleased write ran: %v", err)
		}
	}
	ConfigureChainMutationLease(time.Now().Add(5 * time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := slotAssignmentMutation(ctx, func(context.Context) error { t.Fatal("canceled write ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	err := slotAssignmentMutation(context.Background(), func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Before(time.Unix(0, chainMutationLeaseExpiry.Load())) {
			t.Fatal("write deadline exceeds lease")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSlotWalkerCancellationLeavesCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &testSlotStore{}
	if _, err := walkSlotAssignments(ctx, &testSlotSource{finalized: 2}, store); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if store.exists || len(store.rows) != 0 {
		t.Fatal("canceled walker persisted data")
	}
}
