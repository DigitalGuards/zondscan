package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
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
	}{
		{"empty", evidenceBlock(0, "[]", "[]"), 200, false},
		{"missed", `{"message":"block not found"}`, 404, false},
		{"generic 404", `{"message":"unknown route"}`, 404, true},
		{"unavailable", `{"message":"database unavailable"}`, 503, true},
		{"non-200 malformed body", `{`, 502, true},
		{"bad json", `{`, 200, true},
		{"wrong slot", evidenceBlock(1, "[]", "[]"), 200, true},
		{"non-zond version", strings.Replace(evidenceBlock(0, "[]", "[]"), `"zond"`, `"other"`, 1), 200, false},
		{"missing arrays", evidenceBlock(0, "null", "[]"), 200, true},
		{"optimistic", strings.Replace(evidenceBlock(0, "[]", "[]"), `"version":"zond"`, `"execution_optimistic":true,"version":"zond"`, 1), 200, true},
		{"optimistic invalid block", strings.Replace(evidenceBlock(1, "null", "[]"), `"version":"zond"`, `"execution_optimistic":true,"version":"other"`, 1), 200, true},
		{"invalid root", strings.Replace(evidenceBlock(0, headerProof(), "[]"), testRoot("3"), "0x12", 1), 200, true},
		{"inconsistent headers", evidenceBlock(0, strings.Replace(headerProof(), `"proposer_index":"2"`, `"proposer_index":"3"`, 1), "[]"), 200, true},
		{"invalid conflict", evidenceBlock(0, strings.ReplaceAll(headerProof(), testRoot("4"), testRoot("3")), "[]"), 200, true},
		{"invalid attesting indices", evidenceBlock(0, "[]", strings.Replace(attesterProof(), `["2","3","4"]`, `["2","2","4"]`, 1)), 200, true},
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
	if _, err := client.SlashingEvidence(ctx, 0); !errors.Is(err, context.DeadlineExceeded) || calls != 0 {
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

func TestSlashingEvidenceRetainsValidEventsAcrossFailures(t *testing.T) {
	var slots []int64
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		slot, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
		slots = append(slots, slot)
		switch slot {
		case 9, 16:
			fmt.Fprint(w, evidenceBlock(slot, headerProof(), "[]"))
		case 10:
			fmt.Fprint(w, "{")
		case 12:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
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
	if err == nil || !reflect.DeepEqual(slots, []int64{9, 10, 11, 12, 13, 14, 15, 16}) {
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
	if _, err := client.SlashingEvidence(context.Background(), 2); err == nil {
		t.Fatalf("expected evidence failure: %v", err)
	}
}

func TestSlashingEvidenceTransportFailureIsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	client := NewSlotAssignmentClient(server.URL, models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1000})
	if _, err := client.SlashingEvidence(context.Background(), 0); err == nil {
		t.Fatalf("expected transport failure: %v", err)
	}
}

func slashingFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func changeSlashingFixture(t *testing.T, body []byte, change func(map[string]any)) []byte {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	change(response)
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func legacyFixtureContainer(response map[string]any) map[string]any {
	return response["blockContainers"].([]any)[0].(map[string]any)
}

func legacyFixtureBlock(response map[string]any) map[string]any {
	return legacyFixtureContainer(response)["blindedCapellaBlock"].(map[string]any)["block"].(map[string]any)
}

func slot72SlashingEvents() []models.SlashingEvent {
	return []models.SlashingEvent{
		{ID: "72:proposer:0", Slot: 72, Epoch: 9, Type: "proposer", ValidatorIndices: []string{"14"}, Whistleblower: "9",
			ConflictingRoots: []string{"0x64733d0e885a3b9f8dcbdced0c31b8343a0690da771919cdf4085595a4b7fa56", "0x9f1e53e98fdb425a2d0af8bd3226335502c5b003432bd4fe248c219d1b03c186"}},
		{ID: "72:attester:0", Slot: 72, Epoch: 9, Type: "attester", ValidatorIndices: []string{"1", "12"}, Whistleblower: "9",
			ConflictingRoots: []string{"0xade38db7c2d1e1b65389fd704c6812d2e61371f585dc0c9463fbab08c09469b7", "0xd486b4fc968f2e6f2df8b179de4278245ecfcabe0a75f3d352544c1172af05f9"}},
	}
}

func slot72SlashingClient(t *testing.T, primary, fallback []byte, fallbackStatus int) (*SlotAssignmentClient, *atomic.Int32) {
	t.Helper()
	var fallbackCalls atomic.Int32
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/qrl/v1alpha2/beacon/blocks" {
			fallbackCalls.Add(1)
			if r.URL.RawQuery != "slot=72" {
				t.Errorf("fallback query=%s", r.URL.RawQuery)
			}
			w.WriteHeader(fallbackStatus)
			w.Write(fallback)
			return
		}
		slot, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/qrl/v1/beacon/blocks/"), 10, 64)
		if err != nil {
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if slot == 72 {
			w.Write(primary)
		} else {
			fmt.Fprint(w, evidenceBlock(slot, "[]", "[]"))
		}
	})
	return client, &fallbackCalls
}

func TestSlashingEvidenceProposerFallback(t *testing.T) {
	primary := slashingFixture(t, "v1-block-72.json")
	fallback := slashingFixture(t, "v1alpha2-blocks-slot-72.json")
	var intact slashingBlockResponse
	if err := json.Unmarshal(primary, &intact); err != nil {
		t.Fatal(err)
	}
	proof := &intact.Data.Message.Body.ProposerSlashings[0]
	proof.Header2 = proof.Header1
	first := *proof.Header1
	bodyRoot := sha256.Sum256([]byte("throwaway block body one"))
	first.Message.BodyRoot = "0x" + hex.EncodeToString(bodyRoot[:])
	proof.Header1 = &first
	intactBody, err := json.Marshal(intact)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name              string
		primary, fallback []byte
		wantCalls         int32
	}{
		{"corrupted v1", primary, fallback, 1},
		{"intact v1", intactBody, fallback, 0},
		{"absent second header", changeSlashingFixture(t, primary, func(response map[string]any) {
			proof := response["data"].(map[string]any)["message"].(map[string]any)["body"].(map[string]any)["proposer_slashings"].([]any)[0].(map[string]any)
			delete(proof, "signed_header_2")
		}), fallback, 1},
		{"null first header", changeSlashingFixture(t, intactBody, func(response map[string]any) {
			proof := response["data"].(map[string]any)["message"].(map[string]any)["body"].(map[string]any)["proposer_slashings"].([]any)[0].(map[string]any)
			proof["signed_header_1"] = nil
		}), fallback, 1},
		{"absent first header", changeSlashingFixture(t, intactBody, func(response map[string]any) {
			proof := response["data"].(map[string]any)["message"].(map[string]any)["body"].(map[string]any)["proposer_slashings"].([]any)[0].(map[string]any)
			delete(proof, "signed_header_1")
		}), fallback, 1},
		{"future wrapper", primary, changeSlashingFixture(t, fallback, func(response map[string]any) {
			container := legacyFixtureContainer(response)
			container["futureSignedBlock"] = container["blindedCapellaBlock"]
			delete(container, "blindedCapellaBlock")
		}), 1},
		{"noncanonical sibling", primary, changeSlashingFixture(t, fallback, func(response map[string]any) {
			container := legacyFixtureContainer(response)
			response["blockContainers"] = append(response["blockContainers"].([]any), map[string]any{
				"canonical": false, "blindedCapellaBlock": container["blindedCapellaBlock"],
			})
		}), 1},
		{"unrelated canonical blocks", primary, changeSlashingFixture(t, fallback, func(response map[string]any) {
			for _, metadata := range [][2]string{{"71", "9"}, {"72", "8"}} {
				response["blockContainers"] = append(response["blockContainers"].([]any), map[string]any{
					"canonical": true, "otherBlock": map[string]any{"block": map[string]any{
						"slot": metadata[0], "proposerIndex": metadata[1], "body": map[string]any{},
					}},
				})
			}
		}), 1},
		{"legacy attesters unused", primary, changeSlashingFixture(t, fallback, func(response map[string]any) {
			legacyFixtureBlock(response)["body"].(map[string]any)["attesterSlashings"] = "unused"
		}), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, calls := slot72SlashingClient(t, tc.primary, tc.fallback, http.StatusOK)
			events, err := client.SlashingEvidence(context.Background(), 9)
			if err != nil || calls.Load() != tc.wantCalls || !reflect.DeepEqual(events, slot72SlashingEvents()) {
				t.Fatalf("fallback calls=%d events=%+v err=%v", calls.Load(), events, err)
			}
		})
	}
}

func TestSlashingEvidenceProposerFallbackFailuresKeepAttesters(t *testing.T) {
	primary := slashingFixture(t, "v1-block-72.json")
	fallback := slashingFixture(t, "v1alpha2-blocks-slot-72.json")
	legacyProof := func(response map[string]any) map[string]any {
		return legacyFixtureBlock(response)["body"].(map[string]any)["proposerSlashings"].([]any)[0].(map[string]any)
	}
	for _, tc := range []struct {
		name   string
		status int
		change func(map[string]any)
	}{
		{name: "HTTP error", status: http.StatusServiceUnavailable},
		{name: "missing block HTTP error", status: http.StatusNotFound},
		{name: "invalid JSON"},
		{name: "two canonical containers", change: func(response map[string]any) {
			response["blockContainers"] = append(response["blockContainers"].([]any), legacyFixtureContainer(response))
		}},
		{name: "malformed duplicate canonical container", change: func(response map[string]any) {
			response["blockContainers"] = append(response["blockContainers"].([]any), map[string]any{
				"canonical": true, "otherBlock": map[string]any{"block": map[string]any{
					"slot": "72", "proposerIndex": "9", "body": map[string]any{"proposerSlashings": "invalid"},
				}},
			})
		}},
		{name: "no containers", change: func(response map[string]any) { response["blockContainers"] = []any{} }},
		{name: "noncanonical", change: func(response map[string]any) { legacyFixtureContainer(response)["canonical"] = false }},
		{name: "wrong proposer", change: func(response map[string]any) { legacyFixtureBlock(response)["proposerIndex"] = "8" }},
		{name: "wrong slot", change: func(response map[string]any) { legacyFixtureBlock(response)["slot"] = "71" }},
		{name: "bad base64", change: func(response map[string]any) {
			legacyProof(response)["header1"].(map[string]any)["header"].(map[string]any)["bodyRoot"] = "invalid!"
		}},
		{name: "short root", change: func(response map[string]any) {
			legacyProof(response)["header2"].(map[string]any)["header"].(map[string]any)["parentRoot"] = "AA=="
		}},
		{name: "invalid header number", change: func(response map[string]any) {
			legacyProof(response)["header1"].(map[string]any)["header"].(map[string]any)["slot"] = "invalid"
		}},
		{name: "inconsistent headers", change: func(response map[string]any) {
			legacyProof(response)["header2"].(map[string]any)["header"].(map[string]any)["proposerIndex"] = "13"
		}},
		{name: "identical headers", change: func(response map[string]any) {
			proof := legacyProof(response)
			proof["header2"] = proof["header1"]
		}},
		{name: "missing header", change: func(response map[string]any) { delete(legacyProof(response), "header2") }},
		{name: "missing proposer slashings", change: func(response map[string]any) {
			delete(legacyFixtureBlock(response)["body"].(map[string]any), "proposerSlashings")
		}},
		{name: "empty proposer slashings", change: func(response map[string]any) {
			legacyFixtureBlock(response)["body"].(map[string]any)["proposerSlashings"] = []any{}
		}},
		{name: "changed operation count", change: func(response map[string]any) {
			proof := legacyProof(response)
			legacyFixtureBlock(response)["body"].(map[string]any)["proposerSlashings"] = []any{proof, proof}
		}},
		{name: "ambiguous wrapper", change: func(response map[string]any) {
			container := legacyFixtureContainer(response)
			container["otherBlock"] = container["blindedCapellaBlock"]
		}},
		{name: "incomplete page", change: func(response map[string]any) { response["nextPageToken"] = "next" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, status := fallback, tc.status
			if status == 0 {
				status = http.StatusOK
			}
			if tc.change != nil {
				body = changeSlashingFixture(t, fallback, tc.change)
			} else if tc.status == http.StatusNotFound {
				body = []byte(`{"message":"block not found"}`)
			} else if tc.name == "invalid JSON" {
				body = []byte("{")
			}
			client, calls := slot72SlashingClient(t, primary, body, status)
			events, err := client.SlashingEvidence(context.Background(), 9)
			if err == nil || calls.Load() != 1 || !reflect.DeepEqual(events, slot72SlashingEvents()[1:]) {
				t.Fatalf("fallback calls=%d events=%+v err=%v", calls.Load(), events, err)
			}
		})
	}
}

func TestSlashingFallbackUsesSharedLimiter(t *testing.T) {
	client, calls := slot72SlashingClient(t, slashingFixture(t, "v1-block-72.json"), nil, http.StatusOK)
	client.interval = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	events, err := client.slashingsAtSlot(ctx, 72)
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 0 || !reflect.DeepEqual(events, slot72SlashingEvents()[1:]) {
		t.Fatalf("fallback calls=%d events=%+v err=%v", calls.Load(), events, err)
	}
}

func TestSlashingFallbackUsesSharedTimeout(t *testing.T) {
	primary := slashingFixture(t, "v1-block-72.json")
	client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/qrl/v1/beacon/blocks/72" {
			w.Write(primary)
			return
		}
		<-r.Context().Done()
	})
	client.client.Timeout = 20 * time.Millisecond
	events, err := client.slashingsAtSlot(context.Background(), 72)
	if err == nil || !strings.Contains(err.Error(), "beacon request failed or timed out") || !reflect.DeepEqual(events, slot72SlashingEvents()[1:]) {
		t.Fatalf("events=%+v err=%v", events, err)
	}
}

func TestSlashingFallbackPreservesOperationOrdinals(t *testing.T) {
	primary := changeSlashingFixture(t, slashingFixture(t, "v1-block-72.json"), func(response map[string]any) {
		body := response["data"].(map[string]any)["message"].(map[string]any)["body"].(map[string]any)
		proofs := body["proposer_slashings"].([]any)
		body["proposer_slashings"] = append(proofs, proofs[0])
	})
	fallback := changeSlashingFixture(t, slashingFixture(t, "v1alpha2-blocks-slot-72.json"), func(response map[string]any) {
		body := legacyFixtureBlock(response)["body"].(map[string]any)
		proofs := body["proposerSlashings"].([]any)
		body["proposerSlashings"] = append(proofs, proofs[0])
	})
	client, calls := slot72SlashingClient(t, primary, fallback, http.StatusOK)
	events, err := client.SlashingEvidence(context.Background(), 9)
	want := slot72SlashingEvents()
	second := want[0]
	second.ID = "72:proposer:1"
	want = []models.SlashingEvent{want[0], second, want[1]}
	if err != nil || calls.Load() != 1 || !reflect.DeepEqual(events, want) {
		t.Fatalf("fallback calls=%d events=%+v err=%v", calls.Load(), events, err)
	}
}

func TestSlashingFallbackBlockResponseSizeCap(t *testing.T) {
	primary := slashingFixture(t, "v1-block-72.json")
	fallback := slashingFixture(t, "v1alpha2-blocks-slot-72.json")
	for _, tc := range []struct {
		name     string
		size     int
		tooLarge bool
	}{
		{"over 8 MiB", (8 << 20) + 1, false},
		{"at 64 MiB", 64 << 20, false},
		{"over 64 MiB", (64 << 20) + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testAssignmentClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/qrl/v1/beacon/blocks/72" {
					w.Write(primary)
					return
				}
				w.Write(fallback)
				padding := strings.Repeat(" ", 32<<10)
				for remaining := tc.size - len(fallback); remaining > 0; {
					n := min(remaining, len(padding))
					if _, err := io.WriteString(w, padding[:n]); err != nil {
						return
					}
					remaining -= n
				}
			})
			events, err := client.slashingsAtSlot(context.Background(), 72)
			want := slot72SlashingEvents()
			if tc.tooLarge {
				want = want[1:]
			}
			if (err != nil) != tc.tooLarge || (err != nil && !strings.Contains(err.Error(), "exceeds size limit")) || !reflect.DeepEqual(events, want) {
				t.Fatalf("size=%d events=%+v err=%v", tc.size, events, err)
			}
		})
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
