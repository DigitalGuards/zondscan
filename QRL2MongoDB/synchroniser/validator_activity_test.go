package synchroniser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"

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
	if !reflect.DeepEqual(store.snapshotReads, [][]int64{{1, 2, 3}, {0, 1, 2}, {-1, 0, 1}}) || store.changes["2:slashed:0"].Pubkey != "0xaabb" {
		t.Fatalf("snapshot queries=%v changes=%v", store.snapshotReads, store.changes)
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

type historyBeaconSource struct {
	*historyTestSource
	client *rpc.SlotAssignmentClient
}

func (s historyBeaconSource) SlashingEvidence(ctx context.Context, epoch int64) ([]models.SlashingEvent, error) {
	return s.client.SlashingEvidence(ctx, epoch)
}

func (s historyBeaconSource) EjectionBalance(ctx context.Context) (uint64, error) {
	return s.client.EjectionBalance(ctx)
}

func observeActivityWarnings(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zap.WarnLevel)
	previous := configs.Logger
	configs.Logger = zap.New(core)
	t.Cleanup(func() { configs.Logger = previous })
	return logs
}

func TestActivityWalkerRepairsTransientBlockFetch(t *testing.T) {
	for _, failure := range []string{"HTTP 500", "truncated JSON", "HTML"} {
		t.Run(failure, func(t *testing.T) {
			logs := observeActivityWarnings(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/qrl/v1/beacon/blocks/1" {
					t.Error(r.URL.Path)
				}
				if calls == 1 {
					switch failure {
					case "HTTP 500":
						http.Error(w, "unavailable", http.StatusInternalServerError)
					case "truncated JSON":
						fmt.Fprint(w, `{"data":`)
					case "HTML":
						fmt.Fprint(w, "<html>unavailable</html>")
					}
					return
				}
				header := func(root string) string {
					return fmt.Sprintf(`{"message":{"slot":"0","proposer_index":"0","parent_root":"0x%s","state_root":"0x%s","body_root":"0x%s"}}`, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat(root, 64))
				}
				fmt.Fprintf(w, `{"version":"future","data":{"message":{"slot":"1","proposer_index":"4","body":{"proposer_slashings":[{"signed_header_1":%s,"signed_header_2":%s}],"attester_slashings":[]}}}}`, header("3"), header("4"))
			}))
			defer server.Close()
			timing := historyTestTiming
			timing.SlotsPerEpoch = 1
			source := historyBeaconSource{&historyTestSource{finalized: 1, validators: map[int64][]models.StateValidator{
				0: {historyActivityValidator(false)}, 1: {historyActivityValidator(true)},
			}}, rpc.NewSlotAssignmentClient(server.URL, timing)}
			store := &historyTestStore{}
			if more, err := walkValidatorHistory(context.Background(), source, store, timing, historyTestEjectionBalance); err != nil || more {
				t.Fatalf("more=%t err=%v", more, err)
			}
			snapshot := store.records["1"].SlashingSnapshot
			if store.state.BackfillNext != -1 || !snapshot.EvidenceMissing || len(store.changes) != 2 || len(store.events) != 0 {
				t.Fatalf("state=%+v snapshot=%+v changes=%v events=%v", store.state, snapshot, store.changes, store.events)
			}
			// The marker remains deferred until its retry time is due.
			if _, err := walkValidatorHistory(context.Background(), source, store, timing, historyTestEjectionBalance); err != nil || calls != 1 {
				t.Fatalf("premature repair: calls=%d err=%v", calls, err)
			}
			snapshot.EvidenceRetryAt = time.Now().Unix()
			if _, err := walkValidatorHistory(context.Background(), source, store, timing, historyTestEjectionBalance); err != nil {
				t.Fatal(err)
			}
			if calls != 2 || len(store.events) != 1 || store.events["1:proposer:0"].ValidatorIndices[0] != "0" || snapshot.EvidenceMissing || snapshot.EvidenceRetryAt != 0 || !snapshot.Resolved {
				t.Fatalf("calls=%d snapshot=%+v events=%v", calls, snapshot, store.events)
			}
			if logs.Len() != 1 || store.repairReads != 3 || store.repairWrites != 1 {
				t.Fatalf("logs=%v reads=%d writes=%d", logs.All(), store.repairReads, store.repairWrites)
			}
		})
	}
}

func TestActivityWalkerRepairsCorruptedProposerSlashing(t *testing.T) {
	logs := observeActivityWarnings(t)
	primary, err := os.ReadFile("../rpc/testdata/v1-block-72.json")
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := os.ReadFile("../rpc/testdata/v1alpha2-blocks-slot-72.json")
	if err != nil {
		t.Fatal(err)
	}
	var fallbackCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/qrl/v1/beacon/blocks/72":
			w.Write(primary)
		case "/qrl/v1alpha2/beacon/blocks":
			if r.URL.RawQuery != "slot=72" {
				t.Errorf("fallback query=%s", r.URL.RawQuery)
			}
			if fallbackCalls.Add(1) == 1 {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Write(fallback)
		default:
			slot, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
			if err != nil || slot < 65 || slot > 71 {
				t.Errorf("unexpected request: %s", r.URL.Path)
			}
			http.Error(w, `{"message":"block not found"}`, http.StatusNotFound)
		}
	}))
	defer server.Close()
	source := historyBeaconSource{&historyTestSource{finalized: 8, validators: make(map[int64][]models.StateValidator)}, rpc.NewSlotAssignmentClient(server.URL, historyTestTiming)}
	for epoch := int64(0); epoch <= 9; epoch++ {
		for _, index := range []string{"1", "12", "14"} {
			v := historyActivityValidator(epoch == 9)
			v.Index, v.Validator.Pubkey = index, fmt.Sprintf("0x%04s", index)
			source.validators[epoch*8] = append(source.validators[epoch*8], v)
		}
	}
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	source.finalized = 9
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	snapshot := store.records["9"].SlashingSnapshot
	attester := store.events["72:attester:0"]
	if !snapshot.Resolved || !snapshot.EvidenceMissing || snapshot.EvidenceRetryAt == 0 || len(store.events) != 1 ||
		!reflect.DeepEqual(attester.ValidatorIndices, []string{"1", "12"}) || fallbackCalls.Load() != 1 {
		t.Fatalf("snapshot=%+v events=%v fallback calls=%d", snapshot, store.events, fallbackCalls.Load())
	}
	for _, index := range []string{"1", "12", "14"} {
		if store.changes["9:slashed:"+index].Kind != "slashed" {
			t.Fatalf("missing slashed change for %s", index)
		}
	}
	snapshot.EvidenceRetryAt = time.Now().Unix()
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	proposer := store.events["72:proposer:0"]
	if snapshot.EvidenceMissing || snapshot.EvidenceRetryAt != 0 || !snapshot.Resolved || len(store.events) != 2 ||
		!reflect.DeepEqual(proposer.ValidatorIndices, []string{"14"}) || proposer.Whistleblower != "9" ||
		!reflect.DeepEqual(proposer.ConflictingRoots, []string{"0x64733d0e885a3b9f8dcbdced0c31b8343a0690da771919cdf4085595a4b7fa56", "0x9f1e53e98fdb425a2d0af8bd3226335502c5b003432bd4fe248c219d1b03c186"}) ||
		!reflect.DeepEqual(store.events[attester.ID], attester) || fallbackCalls.Load() != 2 || store.repairWrites != 1 || logs.Len() != 1 {
		t.Fatalf("snapshot=%+v events=%v fallback calls=%d repairs=%d logs=%v", snapshot, store.events, fallbackCalls.Load(), store.repairWrites, logs.All())
	}
}

func TestActivityWalkerFailedRepairDefersWithoutWarning(t *testing.T) {
	logs := observeActivityWarnings(t)
	source := activitySource()
	source.evidence = nil
	source.evidenceErr = errors.New("beacon unavailable")
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	snapshot := store.records["2"].SlashingSnapshot
	snapshot.EvidenceRetryAt = time.Now().Unix()
	before := time.Now().Add(time.Hour).Unix()
	source.finalized = 3
	source.validators[24] = source.validators[16]
	for pass := 0; pass < 2; pass++ {
		if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
			t.Fatal(err)
		}
	}
	if store.state.NewestScheduled != 3 || !snapshot.EvidenceMissing || snapshot.EvidenceRetryAt < before || snapshot.EvidenceRetryAt > time.Now().Add(time.Hour).Unix() {
		t.Fatalf("state=%+v snapshot=%+v", store.state, snapshot)
	}
	if logs.Len() != 1 || !reflect.DeepEqual(source.evidenceEpochs, []int64{2, 2}) || store.repairWrites != 1 {
		t.Fatalf("warnings=%v scans=%v writes=%d", logs.All(), source.evidenceEpochs, store.repairWrites)
	}
}

func TestActivityWalkerRepairStoreFailuresLeaveHistoryAvailable(t *testing.T) {
	for _, failure := range []string{"pending read", "changes read", "repair write"} {
		t.Run(failure, func(t *testing.T) {
			logs := observeActivityWarnings(t)
			source := activitySource()
			source.evidenceErr = errors.New("beacon unavailable")
			store := &historyTestStore{}
			if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
				t.Fatal(err)
			}
			snapshot := store.records["2"].SlashingSnapshot
			snapshot.EvidenceRetryAt = time.Now().Unix()
			source.evidenceErr = nil
			failureErr := errors.New("database unavailable")
			switch failure {
			case "pending read":
				store.pendingErr = failureErr
			case "changes read":
				store.changesErr = failureErr
			case "repair write":
				store.repairErr = failureErr
			}
			source.finalized = 3
			source.validators[24] = source.validators[16]
			if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
				t.Fatal(err)
			}
			if store.state.NewestScheduled != 3 || !snapshot.EvidenceMissing || logs.Len() != 1 {
				t.Fatalf("state=%+v snapshot=%+v logs=%v", store.state, snapshot, logs.All())
			}
			if failure == "changes read" && snapshot.EvidenceRetryAt < time.Now().Add(59*time.Minute).Unix() {
				t.Fatal("failed change read did not defer repair")
			}
		})
	}
}

func TestActivityWalkerCancellationDuringEvidencePreventsWrites(t *testing.T) {
	logs := observeActivityWarnings(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := activitySource()
	source.afterEvidence = cancel
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(ctx, source, store, historyTestTiming, historyTestEjectionBalance); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	snapshot := store.records["2"].SlashingSnapshot
	if store.state.BackfillNext != 1 || len(store.records) != 1 || len(store.events) != 0 || len(store.changes) != 0 || snapshot.Resolved || snapshot.EvidenceMissing || snapshot.EvidenceRetryAt != 0 || logs.Len() != 0 {
		t.Fatalf("cancellation changed activity: state=%+v snapshot=%+v", store.state, snapshot)
	}
}

func TestActivityWalkerLeaseLossOrCancellationDuringRepairPreventsWrites(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprint(shutdown), func(t *testing.T) {
			logs := observeActivityWarnings(t)
			previous := chainMutationLeaseExpiry.Load()
			t.Cleanup(func() { chainMutationLeaseExpiry.Store(previous) })
			ConfigureChainMutationLease(time.Now().Add(time.Minute))
			source := activitySource()
			events := source.evidence
			source.evidence = nil
			source.evidenceErr = errors.New("beacon unavailable")
			store := &historyTestStore{leased: true}
			if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
				t.Fatal(err)
			}
			snapshot := store.records["2"].SlashingSnapshot
			snapshot.EvidenceRetryAt = time.Now().Unix()
			before := *snapshot
			state := store.state
			source.evidence, source.evidenceErr = events, nil
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source.afterEvidence = func() {
				if shutdown {
					cancel()
				} else {
					chainMutationLeaseExpiry.Store(0)
				}
			}
			_, err := walkValidatorHistory(ctx, source, store, historyTestTiming, historyTestEjectionBalance)
			if shutdown && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, *snapshot) || store.repairWrites != 0 || len(store.events) != 0 || store.state != state || logs.Len() != 1 {
				t.Fatalf("repair wrote after lease loss or cancellation: snapshot=%+v writes=%d", snapshot, store.repairWrites)
			}
		})
	}
}

func TestActivityWalkerRepairsOnlyOldestDueEpoch(t *testing.T) {
	source := activitySource()
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	for _, record := range store.records {
		record.SlashingSnapshot.EvidenceMissing = true
		record.SlashingSnapshot.EvidenceRetryAt = time.Now().Unix()
	}
	store.records["0"].SlashingSnapshot.EvidenceRetryAt = time.Now().Add(time.Hour).Unix()
	// Epoch 1 is due but lacks stored changes. It is deferred for one hour;
	// epoch 2 remains available for the next pass.
	source.evidenceEpochs = nil
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if store.repairWrites != 1 || store.records["1"].SlashingSnapshot.EvidenceRetryAt < time.Now().Add(59*time.Minute).Unix() || len(source.evidenceEpochs) != 0 {
		t.Fatal("repair skipped oldest due epoch or processed multiple epochs")
	}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err != nil {
		t.Fatal(err)
	}
	if store.repairWrites != 2 || !reflect.DeepEqual(source.evidenceEpochs, []int64{2}) || store.records["2"].SlashingSnapshot.EvidenceMissing {
		t.Fatal("next due epoch was not repaired")
	}
}

func TestActivityWorkerMissingEjectionBalanceContinuesAndRecovers(t *testing.T) {
	logs := observeActivityWarnings(t)
	validSpec := false
	specReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		specReads++
		if r.URL.Path != "/qrl/v1/config/spec" {
			t.Error(r.URL.Path)
		}
		if validSpec {
			fmt.Fprintf(w, `{"data":{"EJECTION_BALANCE":"%d"}}`, historyTestEjectionBalance)
		} else {
			fmt.Fprint(w, `{"data":{}}`)
		}
	}))
	defer server.Close()
	v := historyActivityValidator(false)
	v.Validator.ExitEpoch, v.Validator.EffectiveBalance = "1", "0"
	source := historyBeaconSource{&historyTestSource{finalized: 1, validators: map[int64][]models.StateValidator{0: {v}, 8: {v}}}, rpc.NewSlotAssignmentClient(server.URL, historyTestTiming)}
	worker := &validatorHistoryWorker{}
	store := &historyTestStore{}
	for pass := 0; pass < 2; pass++ {
		if _, err := worker.pass(context.Background(), source, store, historyTestTiming); err != nil {
			t.Fatal(err)
		}
	}
	if store.state.BackfillNext != -1 || store.changes["1:exited:0"].Kind != "exited" || len(store.changes) != 2 || logs.Len() != 1 || specReads != 2 {
		t.Fatalf("state=%+v changes=%v logs=%v reads=%d", store.state, store.changes, logs.All(), specReads)
	}
	validSpec = true
	v.Index, v.Validator.ExitEpoch = "1", "2"
	source.finalized = 2
	source.validators[16] = append(source.validators[8], v)
	if _, err := worker.pass(context.Background(), source, store, historyTestTiming); err != nil {
		t.Fatal(err)
	}
	if store.changes["2:ejected:1"].Kind != "ejected" || worker.ejectionBalance != historyTestEjectionBalance || logs.Len() != 1 {
		t.Fatalf("changes=%v worker=%+v logs=%v", store.changes, worker, logs.All())
	}
	if _, err := worker.pass(context.Background(), source, store, historyTestTiming); err != nil || specReads != 3 {
		t.Fatalf("spec reads=%d err=%v", specReads, err)
	}
}

func TestActivityWorkerRetriesSpecTransportFailure(t *testing.T) {
	worker := &validatorHistoryWorker{}
	source := activitySource()
	source.specErr = errors.New("beacon request failed or timed out")
	store := &historyTestStore{}
	for pass := 0; pass < 2; pass++ {
		if _, err := worker.pass(context.Background(), source, store, historyTestTiming); !errors.Is(err, source.specErr) {
			t.Fatal(err)
		}
	}
	if store.exists || len(source.slots) != 0 || source.specReads != 2 {
		t.Fatal("transport failure advanced history or skipped retry")
	}
	source.specErr, source.ejectionBalance = nil, historyTestEjectionBalance
	if _, err := worker.pass(context.Background(), source, store, historyTestTiming); err != nil || store.state.BackfillNext != -1 {
		t.Fatalf("recovery state=%+v err=%v", store.state, err)
	}
}

func TestActivityWalkerCursorResume(t *testing.T) {
	source := activitySource()
	missingSlot := int64(8)
	source.failSlot = &missingSlot
	store := &historyTestStore{}
	if _, err := walkValidatorHistory(context.Background(), source, store, historyTestTiming, historyTestEjectionBalance); err == nil {
		t.Fatal("expected unavailable state")
	}
	source.failSlot = nil
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

func TestActivityWalkerEvidenceFailureAdvances(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		partial bool
	}{
		{"permanent structural failure", errors.New("invalid slashing block"), false},
		{"partial structural failure", errors.New("invalid slashing block"), true},
		{"transient failure", errors.New("beacon HTTP 500"), false},
		{"request timeout", context.DeadlineExceeded, false},
		{"request cancellation with active walker", context.Canceled, true},
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
			before := time.Now().Add(15 * time.Minute).Unix()
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
			if !snapshot.Resolved || !snapshot.EvidenceMissing || snapshot.EvidenceRetryAt < before || snapshot.EvidenceRetryAt > time.Now().Add(15*time.Minute).Unix() {
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
			if tc.err != nil && !strings.Contains(warnings[0].ContextMap()["error"].(string), tc.err.Error()) {
				t.Fatalf("missing error text: %v", warnings)
			}
		})
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
