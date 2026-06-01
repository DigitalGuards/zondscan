package models

import (
	"encoding/json"
	"testing"
)

// TestQuantaFromWei locks in exact base-10 wei -> QRL conversion. The whole
// point of carrying wei as an integer string is that none of these go through
// a float64, so 3.3 stays 3.3 and 24-digit values keep every digit.
func TestQuantaFromWei(t *testing.T) {
	cases := []struct {
		name string
		wei  string
		want string
		ok   bool
	}{
		// The headline bug: 3.3 QRL must be exact, never 3.2999999999999998xx.
		{"3.3 QRL", "3300000000000000000", "3.300000000000000000", true},
		{"0.1 QRL", "100000000000000000", "0.100000000000000000", true},
		{"0.33 QRL", "330000000000000000", "0.330000000000000000", true},
		{"6.18 QRL", "6180000000000000000", "6.180000000000000000", true},
		// 1 gwei = 1e9 wei = 1e-9 QRL: smallest-unit precision survives.
		{"1 gwei", "1000000000", "0.000000001000000000", true},
		// Large value (1000+ QRL) round-trips losslessly.
		{"1000 QRL", "1000000000000000000000", "1000.000000000000000000", true},
		// 24-digit wei: every digit preserved (well past float64's ~15-16
		// significant-digit ceiling).
		{"huge mixed", "123456789012345678901234", "123456.789012345678901234", true},
		{"one wei", "1", "0.000000000000000001", true},
		{"zero", "0", "0.000000000000000000", true},
		{"leading/trailing space tolerated", " 3300000000000000000 ", "3.300000000000000000", true},
		{"empty not ok", "", "", false},
		{"decimal input rejected", "3.3", "", false},
		{"hex input rejected", "0xabc", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := quantaFromWei(tc.wei)
			if ok != tc.ok {
				t.Fatalf("quantaFromWei(%q) ok = %v, want %v", tc.wei, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("quantaFromWei(%q) = %q, want %q", tc.wei, got, tc.want)
			}
		})
	}
}

// TestMarshalJSONPrefersWei verifies the response uses the exact wei fields
// when present, regardless of the lossy float column sitting next to them.
func TestMarshalJSONPrefersWei(t *testing.T) {
	tx := TransactionByAddress{
		Amount:      3.3,                   // float64 3.3 is really 3.2999999999999998
		AmountWei:   "3300000000000000000", // exact
		PaidFees:    0.000001,
		PaidFeesWei: "1000000000000", // 0.000001 QRL
		BlockNumber: "0x10",
	}
	got := marshalToStrings(t, tx)
	if got["Amount"] != "3.300000000000000000" {
		t.Errorf("Amount = %q, want 3.300000000000000000", got["Amount"])
	}
	if got["PaidFees"] != "0.000001000000000000" {
		t.Errorf("PaidFees = %q, want 0.000001000000000000", got["PaidFees"])
	}
	if got["BlockNumber"] != "16" {
		t.Errorf("BlockNumber = %q, want 16", got["BlockNumber"])
	}
}

// TestMarshalJSONFallbackCleansLegacyFloat covers documents written before the
// wei fields existed: the float fallback must still strip the IEEE-754 noise
// that produced the reported "3.29999..." display for typical amounts.
func TestMarshalJSONFallbackCleansLegacyFloat(t *testing.T) {
	tx := TransactionByAddress{Amount: 3.3, PaidFees: 6.18}
	got := marshalToStrings(t, tx)
	if got["Amount"] != "3.300000000000000000" {
		t.Errorf("Amount = %q, want 3.300000000000000000", got["Amount"])
	}
	if got["PaidFees"] != "6.180000000000000000" {
		t.Errorf("PaidFees = %q, want 6.180000000000000000", got["PaidFees"])
	}
}

// marshalToStrings marshals tx and returns the subset of top-level JSON fields
// whose values are strings (Amount/PaidFees/BlockNumber/etc.).
func marshalToStrings(t *testing.T, tx TransactionByAddress) map[string]string {
	t.Helper()
	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out := make(map[string]string)
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			out[k] = s
		}
	}
	return out
}
