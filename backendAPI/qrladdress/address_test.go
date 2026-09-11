package qrladdress

import (
	"strings"
	"testing"
)

const checksumA = "QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA"

const (
	knownLower    = "Qd5812f6cf4a0f645aa620cd57319a0ed649dd8f5519a9dde7770ae5b0e49e547985f35eb972a2a07041561aa39c65a3991478f9b1e6749e05277dcf58a9a8b72"
	knownChecksum = "Qd5812F6Cf4a0f645aa620cd57319a0Ed649dd8f5519A9dde7770ae5b0E49e547985f35eB972A2a07041561aa39c65A3991478f9B1e6749e05277dcf58A9A8B72"
)

func TestCanonicalizeAliases(t *testing.T) {
	lower := strings.Repeat("a", HexLength)
	upper := strings.ToUpper(lower)
	for _, input := range []string{
		"Q" + lower,
		"q" + lower,
		"0x" + lower,
		"0X" + lower,
		"Q" + upper,
		checksumA,
		"q" + checksumA[1:],
		"0x" + checksumA[1:],
		"0X" + checksumA[1:],
	} {
		got, ok := Canonicalize(input)
		if !ok {
			t.Fatalf("Canonicalize(%q) rejected a valid alias", input)
		}
		if got != checksumA {
			t.Errorf("Canonicalize(%q) = %q, want %q", input, got, checksumA)
		}
	}
}

func TestCanonicalizeKnownWalletJSVector(t *testing.T) {
	got, ok := Canonicalize(knownLower)
	if !ok {
		t.Fatal("Canonicalize rejected the pinned wallet.js input")
	}
	if got != knownChecksum {
		t.Errorf("Canonicalize known vector = %q, want %q", got, knownChecksum)
	}
}

func TestAddressValidationBoundariesAndChecksum(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"canonical checksum", checksumA, true},
		{"uniform lowercase", "Q" + strings.Repeat("a", HexLength), true},
		{"uniform uppercase", "Q" + strings.Repeat("A", HexLength), true},
		{"invalid mixed-case checksum", "Q" + strings.Repeat("Ab", HexLength/2), false},
		{"invalid 0x mixed-case checksum", "0x" + strings.Repeat("Ab", HexLength/2), false},
		{"Q127", "Q" + strings.Repeat("a", HexLength-1), false},
		{"Q129", "Q" + strings.Repeat("a", HexLength+1), false},
		{"legacy Q40", "Q" + strings.Repeat("a", 40), false},
		{"non-hex", "Q" + strings.Repeat("a", HexLength-1) + "z", false},
		{"bare body", strings.Repeat("a", HexLength), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidAlias(tc.input); got != tc.want {
				t.Errorf("IsValidAlias(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCanonicalInputRequiresUppercaseQPrefix(t *testing.T) {
	lower := strings.Repeat("a", HexLength)
	if !IsValidCanonicalInput("Q" + lower) {
		t.Fatal("uppercase-Q address was rejected")
	}
	for _, input := range []string{"q" + lower, "0x" + lower, "0X" + lower} {
		if IsValidCanonicalInput(input) {
			t.Errorf("IsValidCanonicalInput(%q) = true, want false", input)
		}
	}
}
