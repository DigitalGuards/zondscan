package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"QRL2MongoDB/models"
)

func evidenceBlock(slot int64, proposer, attester string) string {
	return fmt.Sprintf(`{"version":"zond","data":{"message":{"slot":"%d","proposer_index":"7","parent_root":"%s","state_root":"%s","body":{"proposer_slashings":%s,"attester_slashings":%s}}}}`, slot, testRoot("1"), testRoot("2"), proposer, attester)
}

func testRoot(char string) string { return "0x" + strings.Repeat(char, 64) }

func headerProof() string {
	header := func(body string) string {
		return fmt.Sprintf(`{"message":{"slot":"8","proposer_index":"2","parent_root":"%s","state_root":"%s","body_root":"%s"},"signature":"0x00"}`, testRoot("1"), testRoot("2"), testRoot(body))
	}
	return fmt.Sprintf(`[{"signed_header_1":%s,"signed_header_2":%s}]`, header("3"), header("4"))
}

func attesterProof() string {
	attestation := func(indices, root string) string {
		return fmt.Sprintf(`{"attesting_indices":%s,"data":{"slot":"8","index":"0","beacon_block_root":"%s","source":{"epoch":"0","root":"%s"},"target":{"epoch":"1","root":"%s"}},"signature":"0x00"}`, indices, testRoot(root), testRoot("1"), testRoot("2"))
	}
	return fmt.Sprintf(`[{"attestation_1":%s,"attestation_2":%s}]`, attestation(`["2","3","4"]`, "3"), attestation(`["3","4","5"]`, "4"))
}

func TestSlashingEvidenceProposerAttesterAndBoundary(t *testing.T) {
	var slots []int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slot, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
		if err != nil {
			t.Error(r.URL.Path)
		}
		slots = append(slots, slot)
		switch slot {
		case 9:
			fmt.Fprint(w, evidenceBlock(slot, headerProof(), "[]"))
		case 10:
			http.Error(w, `{"code":404,"message":"Could not find requested block"}`, 404)
		case 16:
			fmt.Fprint(w, evidenceBlock(slot, "[]", attesterProof()))
		default:
			fmt.Fprint(w, evidenceBlock(slot, "[]", "[]"))
		}
	}))
	defer server.Close()
	client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1000})
	client.interval = 0
	events, err := client.SlashingEvidence(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(slots, []int64{9, 10, 11, 12, 13, 14, 15, 16}) {
		t.Fatalf("window=%v", slots)
	}
	if len(events) != 2 || events[0].Type != "proposer" || events[0].Slot != 9 || events[0].Epoch != 1 || events[0].Whistleblower != "7" || events[0].ValidatorIndices[0] != "2" {
		t.Fatal(events)
	}
	if events[1].Type != "attester" || events[1].Epoch != 2 || !reflect.DeepEqual(events[1].ValidatorIndices, []string{"3", "4"}) {
		t.Fatal(events)
	}
	for _, event := range events {
		if len(event.ConflictingRoots) != 2 || event.ConflictingRoots[0] == event.ConflictingRoots[1] || len(event.ConflictingRoots[0]) != 66 {
			t.Fatal(event)
		}
	}
}

func TestSlashingEvidenceEmptyEpochAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		fail       bool
		structural bool
	}{
		{"empty", evidenceBlock(0, "[]", "[]"), 200, false, false},
		{"missed", `{"message":"block not found"}`, 404, false, false},
		{"generic 404", `{"message":"unknown route"}`, 404, true, false},
		{"unavailable", `{"message":"database unavailable"}`, 503, true, false},
		{"non-200 malformed body", `{`, 502, true, false},
		{"bad json", `{`, 200, true, true},
		{"wrong slot", evidenceBlock(1, "[]", "[]"), 200, true, true},
		{"non-zond version", strings.Replace(evidenceBlock(0, "[]", "[]"), `"zond"`, `"other"`, 1), 200, true, true},
		{"missing arrays", evidenceBlock(0, "null", "[]"), 200, true, true},
		{"optimistic", strings.Replace(evidenceBlock(0, "[]", "[]"), `"version":"zond"`, `"execution_optimistic":true,"version":"zond"`, 1), 200, true, false},
		{"optimistic invalid block", strings.Replace(evidenceBlock(1, "null", "[]"), `"version":"zond"`, `"execution_optimistic":true,"version":"other"`, 1), 200, true, false},
		{"invalid root", strings.Replace(evidenceBlock(0, headerProof(), "[]"), testRoot("3"), "0x12", 1), 200, true, true},
		{"inconsistent headers", evidenceBlock(0, strings.Replace(headerProof(), `"proposer_index":"2"`, `"proposer_index":"3"`, 1), "[]"), 200, true, true},
		{"invalid conflict", evidenceBlock(0, strings.ReplaceAll(headerProof(), testRoot("4"), testRoot("3")), "[]"), 200, true, true},
		{"invalid attesting indices", evidenceBlock(0, "[]", strings.Replace(attesterProof(), `["2","3","4"]`, `["2","2","4"]`, 1)), 200, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1000})
			client.interval = 0
			events, err := client.SlashingEvidence(context.Background(), 0)
			if (err != nil) != tc.fail || (!tc.fail && len(events) != 0) {
				t.Fatalf("events=%v err=%v", events, err)
			}
			if errors.Is(err, models.ErrStructuralSlashingEvidence) != tc.structural {
				t.Fatalf("structural=%t err=%v", tc.structural, err)
			}
		})
	}
}

func TestSlashingEvidenceUsesSharedLimiterAndCancellation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, evidenceBlock(0, "[]", "[]")) }))
	defer server.Close()
	client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1000})
	client.nextRequest = time.Now().Add(time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.SlashingEvidence(ctx, 0); !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, models.ErrStructuralSlashingEvidence) || calls != 0 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestSlashingEvidenceIntervalWithoutOperations(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slot, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
		calls++
		fmt.Fprint(w, evidenceBlock(slot, "[]", "[]"))
	}))
	defer server.Close()
	client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 4, SecondsPerSlot: 12, GenesisTime: 1000})
	client.interval = 0
	events, err := client.SlashingEvidence(context.Background(), 2)
	if err != nil || len(events) != 0 || calls != 4 {
		t.Fatalf("events=%v calls=%d err=%v", events, calls, err)
	}
}

func TestSlashingEvidenceRetainsValidEventsAcrossStructuralFailures(t *testing.T) {
	var slots []int64
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		slot, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
		slots = append(slots, slot)
		switch slot {
		case 9, 16:
			fmt.Fprint(w, evidenceBlock(slot, headerProof(), "[]"))
		case 10:
			fmt.Fprint(w, "{")
		case 11:
			// Valid operations on either side of a malformed proof survive.
			proof := strings.TrimSuffix(strings.TrimPrefix(headerProof(), "["), "]")
			bad := strings.Replace(proof, testRoot("3"), "0x12", 1)
			fmt.Fprint(w, evidenceBlock(slot, "["+proof+","+bad+","+proof+"]", attesterProof()))
		default:
			fmt.Fprint(w, evidenceBlock(slot, "[]", "[]"))
		}
	})
	events, err := client.SlashingEvidence(context.Background(), 2)
	if !errors.Is(err, models.ErrStructuralSlashingEvidence) || !reflect.DeepEqual(slots, []int64{9, 10, 11, 12, 13, 14, 15, 16}) {
		t.Fatalf("slots=%v err=%v", slots, err)
	}
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	if !reflect.DeepEqual(ids, []string{"9:proposer:0", "11:proposer:0", "11:proposer:2", "11:attester:0", "16:proposer:0"}) {
		t.Fatalf("events=%v", ids)
	}
}

func TestSlashingEvidenceTransientFailureAfterStructuralFailure(t *testing.T) {
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/9") {
			fmt.Fprint(w, "{")
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	if _, err := client.SlashingEvidence(context.Background(), 2); err == nil || errors.Is(err, models.ErrStructuralSlashingEvidence) {
		t.Fatalf("transient failure must retry: %v", err)
	}
}

func TestSlashingEvidenceTransportFailureIsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1000})
	if _, err := client.SlashingEvidence(context.Background(), 0); err == nil || errors.Is(err, models.ErrStructuralSlashingEvidence) {
		t.Fatalf("transport failure must retry: %v", err)
	}
}

func TestSlashingContainerRoots(t *testing.T) {
	// V3 test-network header, with its root reported by the beacon.
	v3Root, err := headerRoot(slashingHeader{
		Slot: "41774", ProposerIndex: "41",
		ParentRoot: "0x4c0a19731b3d3ac69b610f4031f1249f1376209d30b73f51df7e1792547a1621",
		StateRoot:  "0x7b73a47dcd3f6f04199f4f5fcce15579c43d93c8f4f116286b1f126cadbc70c9",
		BodyRoot:   "0xa6de516a259845b44eaa1f24761649d20c91d907f487ad0289dc3d3aaf97eba3",
	})
	if got := "0x" + hex.EncodeToString(v3Root[:]); err != nil || got != "0x16b49aa8a1ff57c9b77779f03a239a7907ba179c473b47ef8aa38fa9da7debb5" {
		t.Fatalf("v3 header root=%s err=%v", got, err)
	}
	var response slashingBlockResponse
	if err := json.Unmarshal([]byte(evidenceBlock(0, headerProof(), attesterProof())), &response); err != nil {
		t.Fatal(err)
	}
	root, err := headerRoot(response.Data.Message.Body.ProposerSlashings[0].Header1.Message)
	if err != nil {
		t.Fatal(err)
	}
	// Independent SSZ vectors for the five-field containers in these fixtures.
	if got := hex.EncodeToString(root[:]); got != "a3f0a913904d283c0230b3a5760b232d5495d4dd0c3aab11141a2cdf72fe115f" {
		t.Fatalf("header root=%s", got)
	}
	root, err = attestationRoot(response.Data.Message.Body.AttesterSlashings[0].Attestation1.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(root[:]); got != "778e308e99ba005d25f1a794ed5008fc2e61d47b3fd1e7b777344842429b6df1" {
		t.Fatalf("attestation root=%s", got)
	}
}
