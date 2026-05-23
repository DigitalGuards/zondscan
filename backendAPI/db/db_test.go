package db

import (
	"reflect"
	"testing"
)

// Tests for the pure-function helpers in this package. The mongo-dependent
// functions need a live connection (see configs.DB) and aren't unit-test
// material; these only cover the deterministic helpers so we get safety net
// on the bits that drive query keys + sorts.

func TestLogIndexSortKey(t *testing.T) {
	// Empty maps to -1 so legacy direct-calldata rows (logIndex == "")
	// sort BEFORE any event-derived row. The whole reason the int64 fan
	// exists, the BSON-string sort iter 5 fixed, is to put "0xa" after
	// "0x9" instead of after "0x10".
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"empty pins legacy rows to front", "", -1},
		{"zero hex", "0x0", 0},
		{"single hex digit", "0x9", 9},
		{"two digit hex", "0xa", 10},
		{"width crossover", "0x10", 16},
		{"uppercase prefix tolerated", "0X1f", 31},
		{"mixed case body", "0xAb", 171},
		{"malformed falls back to 0", "not-hex", 0},
		// The TrimPrefix-then-ParseInt(base 16) path also accepts bare hex
		// (no 0x prefix); in practice the syncer always writes the 0x form
		// but the defensive behaviour is locked in here so a future
		// regression doesn't surprise a caller passing "1234".
		{"naked hex parsed as hex too", "1234", 0x1234},
		{"int64 max sanity", "0x7fffffffffffffff", 0x7fffffffffffffff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := logIndexSortKey(tc.in)
			if got != tc.want {
				t.Errorf("logIndexSortKey(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeAddress(t *testing.T) {
	// The syncer canonicalises addresses to uppercase-Q + lowercase-hex
	// regardless of how the input arrived. Every Mongo query keyed by
	// address goes through this, so it has to be air-tight against the
	// three input forms users + RPC + frontend produce.
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"0x lowercase passes through stripped", "0xabcdef0123456789abcdef0123456789abcdef01", "Qabcdef0123456789abcdef0123456789abcdef01"},
		{"0X uppercase prefix tolerated", "0XABCDEF0123456789ABCDEF0123456789ABCDEF01", "Qabcdef0123456789abcdef0123456789abcdef01"},
		{"Q-prefix already canonical", "Qabcdef0123456789abcdef0123456789abcdef01", "Qabcdef0123456789abcdef0123456789abcdef01"},
		{"q lowercase normalises to Q", "qABCDEF0123456789ABCDEF0123456789ABCDEF01", "Qabcdef0123456789abcdef0123456789abcdef01"},
		{"bare hex gets Q prefix", "ABCDEF0123456789ABCDEF0123456789ABCDEF01", "Qabcdef0123456789abcdef0123456789abcdef01"},
		{"empty becomes Q", "", "Q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeAddress(tc.in)
			if got != tc.want {
				t.Errorf("normalizeAddress(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeAddressBoth(t *testing.T) {
	// Slice form is used by Mongo $in queries; the canonical singleton
	// shape lets us swap it in without touching call sites.
	got := normalizeAddressBoth("0xabc")
	want := []string{"Qabc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalizeAddressBoth = %#v, want %#v", got, want)
	}
}
