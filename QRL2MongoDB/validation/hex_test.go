package validation

import (
	"strings"
	"testing"
)

func TestIsValidAddress(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"QIP-55 Q address", "Q" + strings.Repeat("a", AddressLength), true},
		{"QIP-55 lowercase q address", "q" + strings.Repeat("A", AddressLength), true},
		{"QIP-55 0x address", "0x" + strings.Repeat("1", AddressLength), true},
		{"QIP-55 uppercase 0X address", "0X" + strings.Repeat("AB", AddressLength/2), true},
		{"legacy 20-byte Q address", "Q" + strings.Repeat("a", 40), false},
		{"one hex character short", "Q" + strings.Repeat("a", AddressLength-1), false},
		{"one hex character long", "Q" + strings.Repeat("a", AddressLength+1), false},
		{"bare address body", strings.Repeat("a", AddressLength), false},
		{"non-hex body", "Q" + strings.Repeat("z", AddressLength), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidAddress(tc.in); got != tc.want {
				t.Fatalf("IsValidAddress(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsZeroAddress(t *testing.T) {
	zeroBody := strings.Repeat("0", AddressLength)
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"short Q zero", "Q0", true},
		{"short lowercase q zero", "q0", true},
		{"short 0x zero", "0x0", true},
		{"QIP-55 Q zero", "Q" + zeroBody, true},
		{"QIP-55 lowercase q zero", "q" + zeroBody, true},
		{"QIP-55 0x zero", "0x" + zeroBody, true},
		{"legacy 20-byte zero", "Q" + strings.Repeat("0", 40), false},
		{"bare zero", "0", false},
		{"non-zero QIP-55 address", "Q" + strings.Repeat("0", AddressLength-1) + "1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsZeroAddress(tc.in); got != tc.want {
				t.Fatalf("IsZeroAddress(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
