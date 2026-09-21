package rpc

import (
	"QRL2MongoDB/models"
	"reflect"
	"strings"
	"testing"
)

// Package configs connects to MongoDB at init, so this suite (like the
// token tests) needs MONGOURI pointing at a reachable instance.

// oneQRLWei is 1 QRL expressed as hex wei (1e18).
const oneQRLWei = "0xde0b6b3a7640000"

var (
	traceAlice = "0x" + strings.TrimPrefix(aliceAddr, "Q")
	traceBob   = "0x" + strings.TrimPrefix(bobAddr, "Q")
)

func TestFlattenCallsValueTransfer(t *testing.T) {
	// HTLC-claim shape: outer 0-value CALL into the contract, one nested
	// frame paying the recipient out of contract-held funds.
	tree := []models.Call{
		{
			Type:  "CALL",
			From:  traceAlice,
			To:    traceBob,
			Value: oneQRLWei,
			Gas:   "0x2300",
		},
	}

	got := flattenCalls(tree, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 internal call, got %d", len(got))
	}
	ic := got[0]
	if ic.From != aliceAddr {
		t.Errorf("from not converted to Q-prefix: %s", ic.From)
	}
	if ic.To != bobAddr {
		t.Errorf("to not converted to Q-prefix: %s", ic.To)
	}
	if ic.Value != 1.0 {
		t.Errorf("expected value 1.0 QRL, got %v", ic.Value)
	}
	if !reflect.DeepEqual(ic.TraceAddress, []int{0}) {
		t.Errorf("expected traceAddress [0], got %v", ic.TraceAddress)
	}
	if ic.Input != "0x" || ic.Output != "0x" {
		t.Errorf("expected default input/output 0x, got %q / %q", ic.Input, ic.Output)
	}
	if ic.GasUsed != "0x0" {
		t.Errorf("expected default gasUsed 0x0, got %q", ic.GasUsed)
	}
}

func TestFlattenCallsSkipsZeroValueNoise(t *testing.T) {
	tree := []models.Call{
		{Type: "STATICCALL", From: traceAlice, To: traceBob},
		{Type: "DELEGATECALL", From: traceAlice, To: traceBob, Value: "0x0"},
	}
	if got := flattenCalls(tree, nil); len(got) != 0 {
		t.Fatalf("expected no internal calls for 0-value frames, got %d", len(got))
	}
}

func TestFlattenCallsKeepsCreate(t *testing.T) {
	tree := []models.Call{
		{Type: "CREATE2", From: traceAlice, To: traceBob, Value: "0x0"},
	}
	got := flattenCalls(tree, nil)
	if len(got) != 1 || got[0].Type != "CREATE2" {
		t.Fatalf("expected CREATE2 frame kept despite 0 value, got %v", got)
	}
	if got[0].Value != 0 {
		t.Errorf("expected value 0, got %v", got[0].Value)
	}
}

func TestFlattenCallsSkipsRevertedSubtree(t *testing.T) {
	// The errored frame and its value-bearing child must both be dropped;
	// the sibling after it keeps its own tree index.
	tree := []models.Call{
		{
			Type:  "CALL",
			Error: "execution reverted",
			Value: oneQRLWei,
			From:  traceAlice,
			To:    traceBob,
			Calls: []models.Call{
				{Type: "CALL", Value: oneQRLWei, From: traceAlice, To: traceBob},
			},
		},
		{
			Type:  "CALL",
			Value: oneQRLWei,
			From:  traceAlice,
			To:    traceBob,
		},
	}

	got := flattenCalls(tree, nil)
	if len(got) != 1 {
		t.Fatalf("expected only the healthy sibling, got %d frames", len(got))
	}
	if !reflect.DeepEqual(got[0].TraceAddress, []int{1}) {
		t.Errorf("expected traceAddress [1], got %v", got[0].TraceAddress)
	}
}

func TestFlattenCallsNestedPaths(t *testing.T) {
	// A value transfer two levels deep: parent frame itself carries no
	// value (not persisted) but the path indices must still accumulate.
	tree := []models.Call{
		{
			Type: "CALL",
			From: traceAlice,
			To:   traceBob,
			Calls: []models.Call{
				{Type: "STATICCALL"},
				{
					Type:  "CALL",
					Value: oneQRLWei,
					From:  traceAlice,
					To:    traceBob,
				},
			},
		},
	}

	got := flattenCalls(tree, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 internal call, got %d", len(got))
	}
	if !reflect.DeepEqual(got[0].TraceAddress, []int{0, 1}) {
		t.Errorf("expected traceAddress [0 1], got %v", got[0].TraceAddress)
	}
}

func TestDecodeAddressAndAmountInput(t *testing.T) {
	canonical := "0xa9059cbb" + encodeAddressForABI(aliceAddr) + word("2a")
	address, amount, err := decodeAddressAndAmountInput(canonical)
	if err != nil {
		t.Fatalf("canonical input rejected: %v", err)
	}
	if address != aliceAddr || amount != 42 {
		t.Fatalf("got address=%s amount=%d", address, amount)
	}

	tests := []struct {
		name  string
		input string
	}{
		{name: "short input", input: "0xa9059cbb" + encodeAddressForABI(aliceAddr)},
		{name: "non-hex input", input: "0xa9059cbb" + strings.Repeat("z", 2*abiWordHexLength)},
		{
			name:  "nonzero uint256 high half",
			input: "0xa9059cbb" + encodeAddressForABI(aliceAddr) + "1" + strings.Repeat("0", abiWordHexLength-1),
		},
		{
			name:  "amount exceeds uint64",
			input: "0xa9059cbb" + encodeAddressForABI(aliceAddr) + word("10000000000000000"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := decodeAddressAndAmountInput(tt.input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExactBlockLogsFilterUsesHashSelector(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	filter, err := exactBlockLogsFilter(
		"0x2a",
		strings.ToUpper(blockHash[:2])+blockHash[2:],
		[]string{TransferEventSignature, TransferSingleEventSignature},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := filter["blockHash"]; got != blockHash {
		t.Fatalf("blockHash filter = %v, want %s", got, blockHash)
	}
	if _, exists := filter["fromBlock"]; exists {
		t.Fatalf("exact block filter contains fromBlock: %#v", filter)
	}
	topics, ok := filter["topics"].([][]string)
	if !ok || len(topics) != 1 || len(topics[0]) != 2 {
		t.Fatalf("topics filter = %#v", filter["topics"])
	}

	for _, test := range []struct {
		name        string
		blockNumber string
		blockHash   string
		topic       string
	}{
		{name: "noncanonical number", blockNumber: "0x02a", blockHash: blockHash, topic: TransferEventSignature},
		{name: "short hash", blockNumber: "0x2a", blockHash: "0x12", topic: TransferEventSignature},
		{name: "short topic", blockNumber: "0x2a", blockHash: blockHash, topic: "0x12"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := exactBlockLogsFilter(
				test.blockNumber,
				test.blockHash,
				[]string{test.topic},
			); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
