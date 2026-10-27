package synchroniser

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func historyActivityValidator(slashed bool) models.StateValidator {
	return models.StateValidator{Index: "0", Balance: "40000000000000", Status: "active_ongoing", Validator: models.StateValidatorDetails{
		Pubkey: "0xaabb", EffectiveBalance: "40000000000000", ActivationEligibilityEpoch: "0", ActivationEpoch: "0",
		ExitEpoch: "18446744073709551615", WithdrawableEpoch: "18446744073709551615", Slashed: slashed,
	}}
}

func activitySource() *historyTestSource {
	return &historyTestSource{finalized: 2, validators: map[int64][]models.StateValidator{
		0: {historyActivityValidator(false)}, 8: {historyActivityValidator(false)}, 16: {historyActivityValidator(true)},
	}, evidence: map[int64][]models.SlashingEvent{2: {
		{ID: "15:proposer:0", Slot: 15, Epoch: 1, Type: "proposer", ValidatorIndices: []string{"0"}, Whistleblower: "4", ConflictingRoots: []string{"0xaa", "0xbb"}},
	}}}
}

func TestActivityWalkerReplaysOldCursorAndResolvesDescendingBoundary(t *testing.T) {
	source := activitySource()
	store := &historyTestStore{exists: true, state: validatorHistoryCursor{NewestScheduled: 2, BackfillNext: -1, CatchupNext: -1, CatchupTarget: -1}}
	if more, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil || more {
		t.Fatalf("%t %v", more, err)
	}
	if !reflect.DeepEqual(source.slots, []int64{16, 8, 0}) || !reflect.DeepEqual(source.evidenceEpochs, []int64{2}) {
		t.Fatalf("states=%v evidence=%v", source.slots, source.evidenceEpochs)
	}
	if len(store.changes) != 2 || store.changes["2:slashed:0"].Kind != "slashed" || len(store.events) != 1 || store.state.Version != validatorHistoryVersion {
		t.Fatalf("changes=%v events=%v", store.changes, store.events)
	}
	// A quiet head performs no state or evidence requests.
	source.slots, source.evidenceEpochs = nil, nil
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if len(source.slots)+len(source.evidenceEpochs) != 0 {
		t.Fatal("quiet head fetched data")
	}
	// Catch-up compares against the retained snapshot at the previous boundary.
	source.finalized = 3
	source.validators[24] = []models.StateValidator{historyActivityValidator(true)}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if len(source.evidenceEpochs) != 0 || len(store.events) != 1 {
		t.Fatal("unchanged slashed flag fetched evidence")
	}
}

func TestActivityWalkerEvidenceRetryAndCursorResume(t *testing.T) {
	source := activitySource()
	source.evidenceErr = errors.New("beacon unavailable")
	store := &historyTestStore{}
	for pass := 0; pass < 3; pass++ {
		if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
			t.Fatal("expected retry")
		}
		if store.state.BackfillNext != 1 || len(store.events) != 0 || len(store.changes) != 0 || store.records["2"].SlashingSnapshot.Resolved {
			t.Fatal("transient evidence failure advanced cursor or resolved activity")
		}
	}
	if !reflect.DeepEqual(source.evidenceEpochs, []int64{2, 2, 2}) {
		t.Fatalf("retries=%v", source.evidenceEpochs)
	}
	source.evidenceErr = nil
	store.failCursor = true
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("expected interrupted cursor")
	}
	if store.state.BackfillNext != 1 || len(store.events) != 1 {
		t.Fatal("interruption state")
	}
	source.evidenceEpochs = nil
	store.failCursor = false
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if len(source.evidenceEpochs) != 0 || len(store.events) != 1 || len(store.changes) != 2 {
		t.Fatal("resume duplicated evidence or changes")
	}
}

func TestActivityWalkerStructuralEvidenceFailureAdvances(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		partial bool
	}{
		{"permanent structural failure", models.ErrStructuralSlashingEvidence, false},
		{"partial structural failure", models.ErrStructuralSlashingEvidence, true},
		{"incomplete attribution", nil, true},
		{"no evidence", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			previousLogger := configs.Logger
			configs.Logger = zap.New(core)
			t.Cleanup(func() { configs.Logger = previousLogger })
			source := activitySource()
			source.evidenceErr = tc.err
			if !tc.partial {
				source.evidence = nil
			}
			for _, slot := range []int64{0, 8, 16} {
				v := historyActivityValidator(slot == 16)
				v.Index, v.Validator.Pubkey = "1", "0xccdd"
				source.validators[slot] = append(source.validators[slot], v)
			}
			store := &historyTestStore{}
			for pass := 0; pass < 3; pass++ {
				more, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance)
				if err != nil {
					t.Fatal(err)
				}
				if !more {
					break
				}
			}
			if store.state.BackfillNext != -1 || len(store.records) != 3 || store.records["0"].SlashingSnapshot == nil {
				t.Fatalf("older history stalled: state=%+v records=%v", store.state, store.records)
			}
			for _, id := range []string{"2:slashed:0", "2:slashed:1"} {
				if store.changes[id].Kind != "slashed" {
					t.Fatalf("missing authoritative change %s", id)
				}
			}
			snapshot := store.records["2"].SlashingSnapshot
			if !snapshot.Resolved || !snapshot.EvidenceMissing {
				t.Fatalf("snapshot=%+v", snapshot)
			}
			wantEvents := 0
			if tc.partial {
				wantEvents = 1
			}
			if len(store.events) != wantEvents {
				t.Fatalf("events=%v", store.events)
			}
			source.finalized = 3
			source.validators[24] = source.validators[16]
			if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
				t.Fatal(err)
			}
			if store.state.NewestScheduled != 3 || !reflect.DeepEqual(source.evidenceEpochs, []int64{2}) {
				t.Fatalf("state=%+v evidence scans=%v", store.state, source.evidenceEpochs)
			}
			warnings := logs.All()
			if len(warnings) != 1 || warnings[0].ContextMap()["epoch"] != int64(2) || !reflect.DeepEqual(warnings[0].ContextMap()["validatorIndices"], []interface{}{"0", "1"}) {
				t.Fatalf("warnings=%v", warnings)
			}
		})
	}
}

func TestActivityWalkerRequiresEjectionBalance(t *testing.T) {
	store := &historyTestStore{}
	source := activitySource()
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, 0); err == nil {
		t.Fatal("missing ejection balance accepted")
	}
	if store.exists || len(source.slots) != 0 {
		t.Fatal("history advanced without ejection balance")
	}
}

func TestActivityWalkerLeaseLossDuringEvidencePreventsWrites(t *testing.T) {
	previous := chainMutationLeaseExpiry.Load()
	defer chainMutationLeaseExpiry.Store(previous)
	ConfigureChainMutationLease(time.Now().Add(time.Minute))
	source := activitySource()
	source.afterEvidence = func() { chainMutationLeaseExpiry.Store(0) }
	store := &historyTestStore{leased: true}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("unleased activity accepted")
	}
	if store.state.BackfillNext != 1 || len(store.events) != 0 || len(store.changes) != 0 || store.records["2"].SlashingSnapshot.Resolved {
		t.Fatal("lease loss changed persisted activity")
	}
}

func TestActivityWalkerCatchupSlashingUsesPreviousSnapshot(t *testing.T) {
	source := activitySource()
	source.finalized = 1
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	source.finalized = 2
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 1 || !reflect.DeepEqual(source.evidenceEpochs, []int64{2}) {
		t.Fatalf("events=%v scans=%v", store.events, source.evidenceEpochs)
	}
}
