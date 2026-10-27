package synchroniser

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"QRL2MongoDB/models"
)

const historyTestEjectionBalance uint64 = 20000000000000

var historyTestTiming = models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1790582760}

type historyTestSource struct {
	finalized      int64
	slots          []int64
	failSlot       *int64
	afterFetch     func()
	validators     map[int64][]models.StateValidator
	evidence       map[int64][]models.SlashingEvent
	evidenceEpochs []int64
	evidenceErr    error
	afterEvidence  func()
}

func (s *historyTestSource) FinalizedEpoch(context.Context) (int64, error) { return s.finalized, nil }
func (s *historyTestSource) ValidatorsAtSlot(ctx context.Context, slot int64) ([]models.StateValidator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.slots = append(s.slots, slot)
	if s.afterFetch != nil {
		s.afterFetch()
	}
	if s.failSlot != nil && *s.failSlot == slot {
		return nil, errors.New("state unavailable")
	}
	if values, ok := s.validators[slot]; ok {
		return values, nil
	}
	return []models.StateValidator{}, nil
}

func (s *historyTestSource) SlashingEvidence(ctx context.Context, epoch int64) ([]models.SlashingEvent, error) {
	s.evidenceEpochs = append(s.evidenceEpochs, epoch)
	if s.afterEvidence != nil {
		s.afterEvidence()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]models.SlashingEvent(nil), s.evidence[epoch]...), s.evidenceErr
}

type historyTestStore struct {
	state      validatorHistoryCursor
	exists     bool
	records    map[string]models.ValidatorHistoryRecord
	failCursor bool
	leased     bool
	changes    map[string]models.ValidatorSetChange
	events     map[string]models.SlashingEvent
}

func (s *historyTestStore) Load(context.Context) (validatorHistoryCursor, bool, error) {
	return s.state, s.exists, nil
}
func (s *historyTestStore) Snapshot(_ context.Context, epoch int64) (*models.SlashingSnapshot, error) {
	row := s.records[strconv.FormatInt(epoch, 10)]
	if row.SlashingSnapshot == nil {
		return nil, nil
	}
	copy := *row.SlashingSnapshot
	return &copy, nil
}

func (s *historyTestStore) Save(ctx context.Context, state validatorHistoryCursor, record *models.ValidatorHistoryRecord, activity validatorActivity) error {
	write := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.changes == nil {
			s.changes = make(map[string]models.ValidatorSetChange)
		}
		if s.events == nil {
			s.events = make(map[string]models.SlashingEvent)
		}
		for _, row := range activity.Changes {
			s.changes[row.ID] = row
		}
		for _, row := range activity.Events {
			s.events[row.ID] = row
		}
		if record != nil {
			if s.records == nil {
				s.records = map[string]models.ValidatorHistoryRecord{}
			}
			s.records[record.Epoch] = *record
			for _, resolution := range activity.Resolutions {
				snapshot := s.records[strconv.FormatInt(resolution.Epoch, 10)].SlashingSnapshot
				snapshot.Resolved = true
				snapshot.EvidenceMissing = resolution.EvidenceErr != nil
			}
			if s.failCursor {
				return errors.New("cursor write interrupted")
			}
		}
		s.state, s.exists = state, true
		return nil
	}
	if s.leased {
		return slotAssignmentMutation(ctx, write)
	}
	return write(ctx)
}

func TestHistoryWalkerNewestFirstResumeAndCatchup(t *testing.T) {
	source := &historyTestSource{finalized: 20}
	store := &historyTestStore{}
	more, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance)
	if err != nil || !more || store.state.BackfillNext != 4 {
		t.Fatalf("more=%t state=%+v err=%v", more, store.state, err)
	}
	for i, slot := range source.slots {
		if slot != int64(20-i)*8 {
			t.Fatalf("order=%v", source.slots)
		}
	}
	source.finalized = 22
	source.slots = nil
	if more, err = walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil || more {
		t.Fatalf("catchup=%t,%v", more, err)
	}
	if !reflect.DeepEqual(source.slots, []int64{176, 168, 32, 24, 16, 8, 0}) {
		t.Fatalf("resume=%v", source.slots)
	}
	if len(store.records) != 23 || store.state.BackfillNext != -1 || store.state.NewestScheduled != 22 {
		t.Fatalf("state=%+v count=%d", store.state, len(store.records))
	}
	source.finalized = 24
	source.slots = nil
	if _, err = walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(source.slots, []int64{192, 184}) {
		t.Fatalf("follow=%v", source.slots)
	}
	source.slots = nil
	if _, err = walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil || len(source.slots) != 0 {
		t.Fatalf("quiet head=%v,%v", source.slots, err)
	}
}

func TestHistoryWalkerResumesInterruptedCatchupRange(t *testing.T) {
	store := &historyTestStore{}
	source := &historyTestSource{finalized: 0}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	source.finalized = 40
	if more, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil || !more || store.state.CatchupNext != 24 {
		t.Fatalf("state=%+v err=%v", store.state, err)
	}
	source.slots = nil
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if source.slots[0] != 24*8 || source.slots[15] != 9*8 {
		t.Fatal(source.slots)
	}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if len(store.records) != 41 || store.state.CatchupNext != -1 || store.state.NewestScheduled != 40 {
		t.Fatal(store.state)
	}
}

func TestHistoryWalkerRetriesUnavailableAndPartialWrites(t *testing.T) {
	slot := int64(16)
	source := &historyTestSource{finalized: 3, failSlot: &slot}
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("expected state failure")
	}
	if store.state.BackfillNext != 2 || len(store.records) != 1 {
		t.Fatal(store.state)
	}
	// A new finalized epoch is still serviced while the old state is unavailable.
	source.finalized = 4
	source.slots = nil
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("expected retry")
	}
	if !reflect.DeepEqual(source.slots, []int64{32, 16}) {
		t.Fatal(source.slots)
	}
	source.failSlot = nil
	store.failCursor = true
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("expected cursor failure")
	}
	if store.state.BackfillNext != 2 || len(store.records) != 3 {
		t.Fatal(store.state)
	}
	source.slots = nil
	store.failCursor = false
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(source.slots, []int64{16, 8, 0}) || len(store.records) != 5 {
		t.Fatalf("slots=%v count=%d", source.slots, len(store.records))
	}
}

func TestHistoryWalkerLeaseLossAfterBeaconReadPreventsWrites(t *testing.T) {
	previous := chainMutationLeaseExpiry.Load()
	defer chainMutationLeaseExpiry.Store(previous)
	ConfigureChainMutationLease(time.Now().Add(time.Minute))
	source := &historyTestSource{finalized: 2, afterFetch: func() { chainMutationLeaseExpiry.Store(0) }}
	store := &historyTestStore{leased: true}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("unleased write accepted")
	}
	if len(store.records) != 0 || store.state.BackfillNext != 2 {
		t.Fatal("lease loss changed history or progress")
	}
}

func TestHistoryWalkerCancellationDoesNotAdvance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(ctx, &historyTestSource{finalized: 2}, store, historyTestTiming, historyTestEjectionBalance); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if store.exists || len(store.records) != 0 {
		t.Fatal("cancellation advanced cursor")
	}
}
