package services

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"QRL2MongoDB/models"

	"go.mongodb.org/mongo-driver/bson"
)

const testEjectionBalance uint64 = 20000000000000

func activityValidator() models.StateValidator {
	far := strconv.FormatUint(math.MaxUint64, 10)
	return models.StateValidator{Index: "0", Balance: "40000000000000", Status: "active_ongoing", Validator: models.StateValidatorDetails{
		Pubkey: "0xaabb", EffectiveBalance: "40000000000000", ActivationEligibilityEpoch: "0",
		ActivationEpoch: "0", ExitEpoch: far, WithdrawableEpoch: far,
	}}
}

func TestSlashingSnapshotIsCompactAndUsesCurrentRegistryPubkey(t *testing.T) {
	v := activityValidator()
	v.Validator.Pubkey = "0x" + strings.Repeat("AB", 2592)
	_, older, err := ValidatorChanges(3, []models.StateValidator{v}, testEjectionBalance)
	if err != nil {
		t.Fatal(err)
	}
	// The current epoch's registry contains the validator before it is slashed.
	registry := []models.StateValidator{v}
	v.Validator.Slashed = true
	_, newer, err := ValidatorChanges(4, []models.StateValidator{v}, testEjectionBalance)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bson.Marshal(newer)
	if err != nil || len(encoded) > 200 || strings.Contains(string(encoded), "pubkey") {
		t.Fatalf("snapshot bytes=%d err=%v", len(encoded), err)
	}
	changes, err := SlashingChanges(older, newer, registry)
	if err != nil || len(changes) != 1 || changes[0].Pubkey != strings.ToLower(v.Validator.Pubkey) {
		t.Fatalf("changes=%v err=%v", changes, err)
	}
	if _, err := SlashingChanges(older, newer, nil); err == nil {
		t.Fatal("slashing absent from the registry accepted")
	}
}

func TestValidatorChangeMilestones(t *testing.T) {
	for _, tc := range []struct {
		name  string
		epoch int64
		edit  func(*models.StateValidator)
		want  []string
	}{
		{"activation", 0, func(v *models.StateValidator) {}, []string{"activated"}},
		{"voluntary exit at full balance", 4, func(v *models.StateValidator) { v.Validator.ExitEpoch = "4" }, []string{"exited"}},
		{"ejection at threshold", 4, func(v *models.StateValidator) {
			v.Validator.ExitEpoch = "4"
			v.Validator.EffectiveBalance = "20000000000000"
		}, []string{"ejected"}},
		{"ejection below threshold", 4, func(v *models.StateValidator) {
			v.Validator.ExitEpoch = "4"
			v.Validator.EffectiveBalance = "19999999999999"
		}, []string{"ejected"}},
		{"slashed below threshold", 4, func(v *models.StateValidator) {
			v.Validator.ExitEpoch = "4"
			v.Validator.EffectiveBalance = "19999999999999"
			v.Validator.Slashed = true
		}, []string{"exited", "slashed"}},
		{"withdrawable epoch alone", 4, func(v *models.StateValidator) { v.Validator.WithdrawableEpoch = "4" }, []string{}},
		{"far future exit below threshold", 4, func(v *models.StateValidator) { v.Validator.EffectiveBalance = "1" }, []string{}},
		{"far future", math.MaxInt64, func(v *models.StateValidator) {}, []string{}},
		{"no change", 5, func(v *models.StateValidator) {}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := activityValidator()
			tc.edit(&v)
			changes, snapshot, err := ValidatorChanges(tc.epoch, []models.StateValidator{v}, testEjectionBalance)
			if err != nil {
				t.Fatal(err)
			}
			slashed, err := SlashingChanges(models.SlashingSnapshot{Epoch: tc.epoch - 1}, snapshot, []models.StateValidator{v})
			if err != nil {
				t.Fatal(err)
			}
			changes = append(changes, slashed...)
			kinds := make([]string, 0)
			for _, c := range changes {
				kinds = append(kinds, c.Kind)
				if c.Epoch != tc.epoch || c.ValidatorIndex != "0" || c.Pubkey != "0xaabb" {
					t.Fatal(c)
				}
			}
			if !reflect.DeepEqual(kinds, tc.want) {
				t.Fatalf("got %v want %v", kinds, tc.want)
			}
		})
	}
}

func TestValidatorChangesEjectionBalanceFallback(t *testing.T) {
	v := activityValidator()
	v.Validator.ExitEpoch = "4"
	v.Validator.EffectiveBalance = "2"
	for _, threshold := range []uint64{0, 1, 2} {
		changes, _, err := ValidatorChanges(4, []models.StateValidator{v}, threshold)
		want := "exited"
		if threshold == 2 {
			want = "ejected"
		}
		if err != nil || len(changes) != 1 || changes[0].Kind != want {
			t.Fatalf("threshold=%d changes=%v err=%v", threshold, changes, err)
		}
	}
	v.Validator.EffectiveBalance = "0"
	changes, _, err := ValidatorChanges(4, []models.StateValidator{v}, 0)
	if err != nil || len(changes) != 1 || changes[0].Kind != "exited" {
		t.Fatalf("zero balance fallback: %v %v", changes, err)
	}
}

func TestResolveSlashingEventsRetainsAttributedEvidence(t *testing.T) {
	changes := []models.ValidatorSetChange{{ValidatorIndex: "0"}, {ValidatorIndex: "1"}}
	snapshot := models.SlashingSnapshot{Epoch: 4, Validators: []models.SlashedValidator{{Index: "0"}, {Index: "1"}}}
	events, err := ResolveSlashingEvents([]models.SlashingEvent{
		{ID: "25:proposer:0", Slot: 25, Epoch: 3, ValidatorIndices: []string{"0"}},
	}, changes, snapshot)
	if err == nil || len(events) != 1 || events[0].ID != "25:proposer:0" {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestSlashingTransitionAndEvidenceAttribution(t *testing.T) {
	v := activityValidator()
	_, old, _ := ValidatorChanges(3, []models.StateValidator{v}, testEjectionBalance)
	v.Validator.Slashed = true
	_, current, _ := ValidatorChanges(4, []models.StateValidator{v}, testEjectionBalance)
	changes, err := SlashingChanges(old, current, []models.StateValidator{v})
	if err != nil || len(changes) != 1 || changes[0].Kind != "slashed" || changes[0].Epoch != 4 {
		t.Fatalf("%+v %v", changes, err)
	}
	events, err := ResolveSlashingEvents([]models.SlashingEvent{
		{ID: "25:attester:0", Slot: 25, Epoch: 3, ValidatorIndices: []string{"0", "1"}},
		{ID: "26:attester:0", Slot: 26, Epoch: 3, ValidatorIndices: []string{"0", "2"}},
	}, changes, current)
	if err != nil || len(events) != 1 || !reflect.DeepEqual(events[0].ValidatorIndices, []string{"0"}) {
		t.Fatal(events)
	}
	if _, err := ResolveSlashingEvents(nil, changes, current); err == nil {
		t.Fatal("missing evidence accepted")
	}
	next := current
	next.Epoch++
	changes, err = SlashingChanges(current, next, []models.StateValidator{v})
	if err != nil || len(changes) != 0 {
		t.Fatalf("repeated slashed flag: %+v %v", changes, err)
	}
	if _, err := SlashingChanges(current, models.SlashingSnapshot{Epoch: 5}, []models.StateValidator{v}); err == nil {
		t.Fatal("flag regression accepted")
	}
	if _, err := SlashingChanges(old, next, []models.StateValidator{v}); err == nil {
		t.Fatal("nonadjacent states accepted")
	}
}

func TestValidatorChangesRejectMalformedRecords(t *testing.T) {
	for _, edit := range []func(*models.StateValidator){
		func(v *models.StateValidator) { v.Validator.ExitEpoch = "18446744073709551616" },
		func(v *models.StateValidator) { v.Validator.Pubkey = "invalid" },
		func(v *models.StateValidator) { v.Index = "-1" },
		func(v *models.StateValidator) { v.Validator.Slashed = true; v.Validator.ActivationEpoch = "5" },
	} {
		v := activityValidator()
		edit(&v)
		if _, _, err := ValidatorChanges(4, []models.StateValidator{v}, testEjectionBalance); err == nil {
			t.Fatal("malformed record accepted")
		}
	}
	v := activityValidator()
	if _, _, err := ValidatorChanges(4, []models.StateValidator{v, v}, testEjectionBalance); err == nil {
		t.Fatal("duplicate index accepted")
	}
}
