package rpc

import (
	"QRL2MongoDB/models"
	"reflect"
	"testing"
)

// Package configs connects to MongoDB at init, so this suite (like the
// token tests) needs MONGOURI pointing at a reachable instance.

// oneQRLWei is 1 QRL expressed as hex wei (1e18).
const oneQRLWei = "0xde0b6b3a7640000"

func TestFlattenCallsValueTransfer(t *testing.T) {
	// HTLC-claim shape: outer 0-value CALL into the contract, one nested
	// frame paying the recipient out of contract-held funds.
	tree := []models.Call{
		{
			Type:  "CALL",
			From:  "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee",
			To:    "0x79b662ce3d663643df4454a8ba3f532c0de6887f",
			Value: oneQRLWei,
			Gas:   "0x2300",
		},
	}

	got := flattenCalls(tree, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 internal call, got %d", len(got))
	}
	ic := got[0]
	if ic.From != "Q94cd8e406d2bb4ea251dce3f0558941f2ac056ee" {
		t.Errorf("from not converted to Q-prefix: %s", ic.From)
	}
	if ic.To != "Q79b662ce3d663643df4454a8ba3f532c0de6887f" {
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
		{Type: "STATICCALL", From: "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee", To: "0x79b662ce3d663643df4454a8ba3f532c0de6887f"},
		{Type: "DELEGATECALL", From: "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee", To: "0x79b662ce3d663643df4454a8ba3f532c0de6887f", Value: "0x0"},
	}
	if got := flattenCalls(tree, nil); len(got) != 0 {
		t.Fatalf("expected no internal calls for 0-value frames, got %d", len(got))
	}
}

func TestFlattenCallsKeepsCreate(t *testing.T) {
	tree := []models.Call{
		{Type: "CREATE2", From: "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee", To: "0x79b662ce3d663643df4454a8ba3f532c0de6887f", Value: "0x0"},
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
			From:  "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee",
			To:    "0x79b662ce3d663643df4454a8ba3f532c0de6887f",
			Calls: []models.Call{
				{Type: "CALL", Value: oneQRLWei, From: "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee", To: "0x79b662ce3d663643df4454a8ba3f532c0de6887f"},
			},
		},
		{
			Type:  "CALL",
			Value: oneQRLWei,
			From:  "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee",
			To:    "0x79b662ce3d663643df4454a8ba3f532c0de6887f",
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
			From: "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee",
			To:   "0x79b662ce3d663643df4454a8ba3f532c0de6887f",
			Calls: []models.Call{
				{Type: "STATICCALL"},
				{
					Type:  "CALL",
					Value: oneQRLWei,
					From:  "0x94cd8e406d2bb4ea251dce3f0558941f2ac056ee",
					To:    "0x79b662ce3d663643df4454a8ba3f532c0de6887f",
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
