package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"QRL2MongoDB/models"
)

func testAssignmentClient(t *testing.T, handler http.HandlerFunc) *SlotAssignmentClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewSlotAssignmentClient(server.URL)
	client.interval = 0
	return client
}

func dutyResponse(epoch int64) any {
	rows := []map[string]string{}
	for slot := epoch * 8; slot < (epoch+1)*8; slot++ {
		if slot == 0 {
			continue
		}
		rows = append(rows, map[string]string{"slot": fmt.Sprint(slot), "validator_index": "0", "pubkey": "0xaabb"})
	}
	return map[string]any{"data": rows}
}

func TestProposerDutiesHistoricalAndGenesis(t *testing.T) {
	for _, epoch := range []int64{0, 30} {
		t.Run(fmt.Sprint(epoch), func(t *testing.T) {
			calls := 0
			client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != fmt.Sprintf("/qrl/v1/validator/duties/proposer/%d", epoch) {
					t.Errorf("path = %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(dutyResponse(epoch))
			})
			rows, err := client.Assignments(context.Background(), epoch)
			if err != nil || len(rows) != 8 || calls != 1 {
				t.Fatalf("rows = %v, error = %v, calls = %d", rows, err, calls)
			}
			if epoch == 0 && rows[0].ValidatorIndex != "" {
				t.Fatal("invented a genesis proposer")
			}
		})
	}
}

func TestAssignmentsFallbackPaginatesAndResolvesPubkeys(t *testing.T) {
	keyLookups, pages := 0, 0
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/qrl/v1/validator/duties/proposer/4":
			http.Error(w, "could not find state", 500)
		case "/qrl/v1alpha1/validators/assignments":
			pages++
			if r.URL.Query().Get("epoch") != "4" || r.URL.Query().Get("page_size") != "100" {
				t.Error("missing assignment query bounds")
			}
			slots, next := []string{"32", "33", "34", "35"}, "1"
			if r.URL.Query().Get("page_token") == "1" {
				slots, next = []string{"36", "37", "38", "39"}, ""
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"epoch": "4", "nextPageToken": next, "assignments": []any{map[string]any{"validatorIndex": "0", "proposerSlots": slots}}})
		case "/qrl/v1/beacon/states/head/validators/0":
			keyLookups++
			fmt.Fprint(w, `{"data":{"index":"0","validator":{"pubkey":"0xaabb"}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	rows, err := client.Assignments(context.Background(), 4)
	if err != nil || len(rows) != 8 || pages != 2 || keyLookups != 1 {
		t.Fatalf("rows=%v error=%v pages=%d lookups=%d", rows, err, pages, keyLookups)
	}
	for _, row := range rows {
		if row.ValidatorIndex != "0" || row.Pubkey != "0xaabb" {
			t.Fatalf("row = %+v", row)
		}
	}
}

func TestAssignmentsOnlySkipExplicitMissingHistory(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		status      int
		unavailable bool
	}{
		{"pruned", "state has been pruned", 500, true},
		{"missing state", "state not found", 404, true},
		{"pruned replay", "could not replay all blocks: no blocks found in db below slot", 500, true},
		{"unavailable slot", "cannot retrieve data for slot 16", 500, true},
		{"missing route", "404 page not found", 404, false},
		{"node error", "internal server error", 500, false},
		{"rate limit", "state not found", 429, false},
		{"bad json", "{", 200, false},
		{"incomplete response", `{"data":[]}`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) })
			_, err := client.Assignments(context.Background(), 2)
			if err == nil || errors.Is(err, ErrEpochUnavailable) != tc.unavailable {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDutyValidationRejectsIncompleteConflictingAndInvalidData(t *testing.T) {
	valid := make([]models.SlotAssignment, 8)
	for i := range valid {
		valid[i] = models.SlotAssignment{Epoch: 1, Slot: int64(8 + i), ValidatorIndex: "0", Pubkey: "0xaabb"}
	}
	for _, mutate := range []func([]models.SlotAssignment) []models.SlotAssignment{
		func(r []models.SlotAssignment) []models.SlotAssignment { return r[:7] },
		func(r []models.SlotAssignment) []models.SlotAssignment { r[1].Slot = 8; return r },
		func(r []models.SlotAssignment) []models.SlotAssignment { r[1].Slot = 16; return r },
		func(r []models.SlotAssignment) []models.SlotAssignment { r[1].Epoch = 2; return r },
		func(r []models.SlotAssignment) []models.SlotAssignment { r[1].ValidatorIndex = ""; return r },
		func(r []models.SlotAssignment) []models.SlotAssignment { r[1].Pubkey = "xyz"; return r },
	} {
		if _, err := ValidateSlotAssignments(1, mutate(append([]models.SlotAssignment{}, valid...))); err == nil {
			t.Fatal("invalid duties accepted")
		}
	}
}

func TestFallbackRejectsRepeatedPageToken(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/duties/") {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"epoch":"1","nextPageToken":"1","assignments":[]}`)
	})
	if _, err := client.Assignments(context.Background(), 1); err == nil || errors.Is(err, ErrEpochUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestMissingHeadPubkeyKeepsHistoricalEpochRetryable(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/assignments") {
			fmt.Fprint(w, `{"epoch":"1","assignments":[{"validatorIndex":"0","proposerSlots":["8","9","10","11","12","13","14","15"]}]}`)
			return
		}
		http.Error(w, "state not found", 404)
	})
	if _, err := client.Assignments(context.Background(), 1); err == nil || errors.Is(err, ErrEpochUnavailable) {
		t.Fatalf("pubkey failure skipped epoch: %v", err)
	}
}

func TestPartialFallbackFailureKeepsEpochRetryable(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/assignments") && r.URL.Query().Get("page_token") == "" {
			fmt.Fprint(w, `{"epoch":"1","nextPageToken":"1","assignments":[{"validatorIndex":"0","proposerSlots":["8"]}]}`)
			return
		}
		http.Error(w, "state not found", 404)
	})
	if _, err := client.Assignments(context.Background(), 1); err == nil || errors.Is(err, ErrEpochUnavailable) {
		t.Fatalf("partial fallback skipped epoch: %v", err)
	}
}

func TestBeaconRequestsRespectTimeoutAndRateCancellation(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	client.client.Timeout = 20 * time.Millisecond
	start := time.Now()
	if _, err := client.GenesisTime(context.Background()); err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > time.Second {
		t.Fatal("request timeout was unbounded")
	}
	client.nextRequest = time.Now().Add(time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.GenesisTime(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("rate wait = %v", err)
	}
}
