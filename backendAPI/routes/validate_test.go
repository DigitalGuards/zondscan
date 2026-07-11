package routes

import "testing"

// These cover the path-param format guards added to reject malformed
// input at the route boundary (HTTP 400) before it reaches the db layer.
// The accepted shapes mirror the frontend search resolver
// (ExplorerFrontend/app/lib/searchResolver.ts).

func TestIsValidTxHash(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical 0x + 64 hex", "0x" + repeat("a", 64), true},
		{"uppercase hex body", "0x" + repeat("AB", 32), true},
		{"missing 0x prefix", repeat("a", 64), false},
		{"one char short", "0x" + repeat("a", 63), false},
		{"one char too long", "0x" + repeat("a", 65), false},
		{"non-hex char", "0x" + repeat("g", 64), false},
		{"address mistaken for hash", "Q" + repeat("a", 40), false},
		{"empty", "", false},
		{"mongo-operator injection attempt", `0x"; return true; //`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidTxHash(tc.in); got != tc.want {
				t.Errorf("isValidTxHash(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsValidAddressParam(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"Q + 40 hex", "Q" + repeat("a", 40), true},
		{"lowercase q + 40 hex", "q" + repeat("a", 40), true},
		{"0x + 40 hex contract form", "0x" + repeat("a", 40), true},
		{"uppercase 0X prefix", "0X" + repeat("AB", 20), true},
		{"bare 40 hex rejected", repeat("a", 40), false},
		{"too short", "Q" + repeat("a", 39), false},
		{"too long", "Q" + repeat("a", 41), false},
		{"tx hash rejected", "0x" + repeat("a", 64), false},
		{"non-hex body", "Q" + repeat("z", 40), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidAddressParam(tc.in); got != tc.want {
				t.Errorf("isValidAddressParam(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsValidValidatorID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"decimal index", "1", true},
		{"large decimal index", "1048576", true},
		{"zero index", "0", true},
		{"bare hex pubkey", repeat("ab", 48), true},
		{"0x-prefixed hex pubkey", "0x" + repeat("ab", 48), true},
		{"odd-length hex rejected", "abc", false},
		{"non-hex rejected", "xyz", false},
		{"empty", "", false},
		{"injection attempt", `{"$ne":null}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidValidatorID(tc.in); got != tc.want {
				t.Errorf("isValidValidatorID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// repeat returns s concatenated n times. Local helper so the table cases
// stay readable without importing strings.Repeat at every call site.
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
