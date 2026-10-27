package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"QRL2MongoDB/models"
)

func TestBeaconEjectionBalance(t *testing.T) {
	for _, tc := range []struct {
		name, spec string
		want       uint64
	}{
		{"v3 test network", `{"data":{"EJECTION_BALANCE":"20000000000000","MAX_EFFECTIVE_BALANCE":"40000000000000"}}`, 20000000000000},
		{"other network", `{"data":{"EJECTION_BALANCE":"1"}}`, 1},
		{"missing", `{"data":{}}`, 0},
		{"empty", `{"data":{"EJECTION_BALANCE":""}}`, 0},
		{"zero", `{"data":{"EJECTION_BALANCE":"0"}}`, 0},
		{"negative", `{"data":{"EJECTION_BALANCE":"-1"}}`, 0},
		{"fractional", `{"data":{"EJECTION_BALANCE":"1.5"}}`, 0},
		{"number", `{"data":{"EJECTION_BALANCE":20000000000000}}`, 0},
		{"overflow", `{"data":{"EJECTION_BALANCE":"18446744073709551616"}}`, 0},
		{"hex", `{"data":{"EJECTION_BALANCE":"0x20"}}`, 0},
		{"whitespace", `{"data":{"EJECTION_BALANCE":" 20"}}`, 0},
		{"plus", `{"data":{"EJECTION_BALANCE":"+20"}}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/qrl/v1/config/spec" {
					t.Error(r.URL.Path)
				}
				fmt.Fprint(w, tc.spec)
			})
			value, err := client.EjectionBalance(context.Background())
			if value != tc.want || (err == nil) != (tc.want > 0) {
				t.Fatalf("value=%d err=%v", value, err)
			}
		})
	}
}

func TestBeaconSpecAndGenesisParsing(t *testing.T) {
	for _, tc := range []struct {
		name, spec, genesis string
		valid               bool
	}{
		{"network", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"60"}}`, `{"data":{"genesis_time":"1790582760"}}`, true},
		{"other network", `{"data":{"SLOTS_PER_EPOCH":"32","SECONDS_PER_SLOT":"12"}}`, `{"data":{"genesis_time":"1790582760"}}`, true},
		{"missing spec", `{"data":{}}`, `{"data":{"genesis_time":"1790582760"}}`, false},
		{"zero slots", `{"data":{"SLOTS_PER_EPOCH":"0","SECONDS_PER_SLOT":"60"}}`, `{"data":{"genesis_time":"1790582760"}}`, false},
		{"negative seconds", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"-1"}}`, `{"data":{"genesis_time":"1790582760"}}`, false},
		{"number spec", `{"data":{"SLOTS_PER_EPOCH":8,"SECONDS_PER_SLOT":"60"}}`, `{"data":{"genesis_time":"1790582760"}}`, false},
		{"overflow", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"9223372036854775807"}}`, `{"data":{"genesis_time":"1790582760"}}`, false},
		{"missing genesis", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"60"}}`, `{"data":{}}`, false},
		{"zero genesis", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"60"}}`, `{"data":{"genesis_time":"0"}}`, false},
		{"invalid genesis", `{"data":{"SLOTS_PER_EPOCH":"8","SECONDS_PER_SLOT":"60"}}`, `{"data":{"genesis_time":"18446744073709551615"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/qrl/v1/config/spec":
					fmt.Fprint(w, tc.spec)
				case "/qrl/v1/beacon/genesis":
					fmt.Fprint(w, tc.genesis)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
			})
			timing, err := client.ChainTiming(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("timing=%+v error=%v", timing, err)
			}
			if tc.valid && timing.GenesisTime != 1790582760 {
				t.Fatal(timing)
			}
		})
	}
}

func TestBeaconHeadUsesSlotAndV1Finality(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/qrl/v1/beacon/headers/head":
			fmt.Fprint(w, `{"data":{"header":{"message":{"slot":"40007"}}}}`)
		case "/qrl/v1/beacon/states/head/finality_checkpoints":
			fmt.Fprint(w, `{"data":{"previous_justified":{"epoch":"4997"},"current_justified":{"epoch":"4999"},"finalized":{"epoch":"4998"}}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	head, err := client.ChainHead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if head.HeadEpoch != "5000" || head.HeadSlot != "40007" || head.FinalizedEpoch != "4998" || head.FinalizedSlot != "39984" || head.JustifiedEpoch != "4999" {
		t.Fatalf("head=%+v", head)
	}
}

func TestBeaconStateValidatorsShapeAndMissingState(t *testing.T) {
	const validator = `{"index":"0","balance":"40000000000001","status":"active_ongoing","validator":{"pubkey":"0xaabb","withdrawal_recipient":"0xccdd","effective_balance":"40000000000000","slashed":false,"activation_eligibility_epoch":"0","activation_epoch":"0","exit_epoch":"18446744073709551615","withdrawable_epoch":"18446744073709551615"}}`
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"data":[` + validator + `]}`, true}, {`{"data":[]}`, true},
		{`{}`, false}, {`{"data":null}`, false},
		{`{"execution_optimistic":true,"data":[` + validator + `]}`, false},
		{`{"data":[` + validator + `,` + validator + `]}`, false},
		{`{"data":[` + strings.Replace(validator, "active_ongoing", "unknown", 1) + `]}`, false},
	} {
		client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/qrl/v1/beacon/states/38000/validators" {
				t.Error(r.URL.Path)
			}
			fmt.Fprint(w, tc.body)
		})
		rows, err := client.ValidatorsAtSlot(context.Background(), 38000)
		if (err == nil) != tc.valid {
			t.Fatalf("rows=%+v error=%v", rows, err)
		}
		if len(rows) == 1 && (rows[0].Validator.WithdrawalRecipient != "0xccdd" || rows[0].Validator.ExitEpoch != "18446744073709551615") {
			t.Fatal(rows)
		}
	}
}

func TestSlotAssignmentsUseConfiguredEpochLength(t *testing.T) {
	timing := models.ChainTiming{SlotsPerEpoch: 4, SecondsPerSlot: 12, GenesisTime: 1790582760}
	rows := []models.SlotAssignment{}
	for slot := int64(8); slot < 12; slot++ {
		rows = append(rows, models.SlotAssignment{Epoch: 2, Slot: slot, ValidatorIndex: "1", Pubkey: "0xaabb"})
	}
	got, err := ValidateSlotAssignments(timing, 2, rows)
	if err != nil || len(got) != 4 {
		t.Fatalf("rows=%+v err=%v", got, err)
	}
}

func TestBeaconStartupDiscoveryReportsUnavailableSpec(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	if _, err := client.ChainTiming(context.Background()); err == nil || !strings.Contains(err.Error(), "read beacon spec") {
		t.Fatalf("error=%v", err)
	}
}
