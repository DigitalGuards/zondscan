package rpc

import (
	"math/big"
	"strings"
	"testing"
)

func TestEncodeAddressForABI(t *testing.T) {
	aliceRaw := strings.TrimPrefix(strings.ToLower(aliceAddr), "q")
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "Q-prefix lowercase",
			addr: aliceAddr,
			want: aliceRaw,
		},
		{
			name: "0x-prefix uppercase canonicalised to lowercase",
			addr: "0x" + strings.ToUpper(aliceRaw),
			want: aliceRaw,
		},
		{
			name: "no prefix",
			addr: aliceRaw,
			want: aliceRaw,
		},
		{
			name: "legacy 20-byte address rejected",
			addr: "Q" + strings.Repeat("a", 40),
			want: "",
		},
		{
			name: "full-width non-hex address rejected",
			addr: "Q" + strings.Repeat("a", abiWordHexLength-1) + "z",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeAddressForABI(tt.addr)
			if got != tt.want {
				t.Errorf("encodeAddressForABI(%s)\n got %s\nwant %s", tt.addr, got, tt.want)
			}
			wantLength := abiWordHexLength
			if tt.want == "" {
				wantLength = 0
			}
			if len(got) != wantLength {
				t.Errorf("encoded length = %d, want %d", len(got), wantLength)
			}
		})
	}
}

func TestEncodeUint256ForABI(t *testing.T) {
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	tests := []struct {
		name    string
		v       *big.Int
		want    string
		wantErr bool
	}{
		{
			name: "zero",
			v:    big.NewInt(0),
			want: strings.Repeat("0", abiWordHexLength),
		},
		{
			name: "42",
			v:    big.NewInt(42),
			want: strings.Repeat("0", abiWordHexLength-2) + "2a",
		},
		{
			name: "uint256 max",
			v:    maxUint256,
			want: strings.Repeat("0", abiWordHexLength-uint256HexLength) + strings.Repeat("f", uint256HexLength),
		},
		{
			name:    "nil",
			v:       nil,
			wantErr: true,
		},
		{
			name:    "negative",
			v:       big.NewInt(-1),
			wantErr: true,
		},
		{
			name:    "exceeds 32 bytes",
			v:       new(big.Int).Lsh(big.NewInt(1), 256),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeUint256ForABI(tt.v)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseAddressFromWord(t *testing.T) {
	aliceRaw := strings.TrimPrefix(strings.ToLower(aliceAddr), "q")

	tests := []struct {
		name     string
		result   string
		wantAddr string
		wantErr  bool
	}{
		{
			name:     "happy path: full-width Alice address",
			result:   "0x" + aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:     "zero address returns empty (no owner)",
			result:   "0x" + strings.Repeat("0", abiWordHexLength),
			wantAddr: "",
		},
		{
			name:     "no 0x prefix accepted",
			result:   aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:    "too-short response",
			result:  "0x" + aliceRaw[:abiWordHexLength-1],
			wantErr: true,
		},
		{
			name:    "trailing word is rejected",
			result:  "0x" + strings.Repeat("0", abiWordHexLength) + aliceRaw,
			wantErr: true,
		},
		{
			name:    "non-hex word",
			result:  "0x" + "z" + aliceRaw[1:],
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAddressFromWord(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.EqualFold(got, tt.wantAddr) {
				t.Errorf("got %s, want %s", got, tt.wantAddr)
			}
		})
	}
}

func TestParseUint256FromWord(t *testing.T) {
	maxHex := strings.Repeat("f", 64)
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			name:   "happy path: 100",
			result: "0x" + word("64"),
			want:   "100",
		},
		{
			name:   "zero",
			result: "0x" + strings.Repeat("0", abiWordHexLength),
			want:   "0",
		},
		{
			name:   "uint256 max",
			result: "0x" + word(maxHex),
			want:   maxUint256.String(),
		},
		{
			name:    "too short",
			result:  "0x" + word("64")[:abiWordHexLength-1],
			wantErr: true,
		},
		{
			name:    "trailing word",
			result:  "0x" + word("0") + word("64"),
			wantErr: true,
		},
		{
			name:    "nonzero high 32 bytes",
			result:  "0x1" + strings.Repeat("0", abiWordHexLength-1),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUint256FromWord(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got.String(), tt.want)
			}
		})
	}
}

// Ensure encodeAddressForABI + parseAddressFromWord round-trip cleanly.
func TestAddressWordRoundTrip(t *testing.T) {
	for _, addr := range []string{aliceAddr, bobAddr, opAddr} {
		encoded := encodeAddressForABI(addr)
		decoded, err := parseAddressFromWord("0x" + encoded)
		if err != nil {
			t.Fatalf("decode %s: %v", addr, err)
		}
		if !strings.EqualFold(decoded, addr) {
			t.Errorf("round-trip: got %s, want %s", decoded, addr)
		}
	}
}

func TestFixedValueWordEncoding(t *testing.T) {
	encoded := encodeBytes4ForABI(InterfaceIDERC721)
	if len(encoded) != abiWordHexLength {
		t.Fatalf("bytes4 word length = %d, want %d", len(encoded), abiWordHexLength)
	}
	if encoded[:8] != "80ac58cd" || strings.Trim(encoded[8:], "0") != "" {
		t.Fatalf("bytes4 is not left-aligned: %s", encoded)
	}

	boolTests := []struct {
		name      string
		word      string
		wantValue bool
		wantValid bool
	}{
		{name: "true", word: "0x" + word("1"), wantValue: true, wantValid: true},
		{name: "false", word: "0x" + word("0"), wantValid: true},
		{name: "invalid scalar", word: "0x" + word("2")},
		{name: "nonzero high bytes", word: "0x1" + strings.Repeat("0", abiWordHexLength-1)},
		{name: "short word", word: "0x01"},
		{name: "trailing word", word: "0x" + word("0") + word("1")},
	}
	for _, tt := range boolTests {
		t.Run(tt.name, func(t *testing.T) {
			value, valid := parseBoolFromWord(tt.word)
			if value != tt.wantValue || valid != tt.wantValid {
				t.Fatalf("got value=%v valid=%v, want value=%v valid=%v", value, valid, tt.wantValue, tt.wantValid)
			}
		})
	}
}

func TestParseDynamicString(t *testing.T) {
	// Helper: build an ABI-encoded dynamic string from a Go string.
	// Layout: [offset=0x40 || length || data right-padded to 64B].
	encode := func(s string) string {
		raw := []byte(s)
		length := len(raw)
		// Right-pad to the next 64-byte boundary.
		padLen := (abiWordBytes - length%abiWordBytes) % abiWordBytes
		padded := make([]byte, length+padLen)
		copy(padded, raw)
		offsetWord := word("40")
		lengthWord := word(new(big.Int).SetInt64(int64(length)).Text(16))
		return "0x" + offsetWord + lengthWord + hexEncode(padded)
	}

	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			name:   "happy path short",
			result: encode("ipfs://Qm.../"),
			want:   "ipfs://Qm.../",
		},
		{
			name:   "happy path 32 bytes exactly",
			result: encode("0123456789abcdef0123456789abcdef"),
			want:   "0123456789abcdef0123456789abcdef",
		},
		{
			name:   "happy path long URI",
			result: encode("https://example.com/collections/my-nfts/metadata.json?v=2026"),
			want:   "https://example.com/collections/my-nfts/metadata.json?v=2026",
		},
		{
			name:   "empty string (method exists, returned empty)",
			result: "0x" + word("40") + word("0"),
			want:   "",
		},
		{
			name:   "all-zero payload returns empty (no error)",
			result: "0x" + strings.Repeat("0", 2*abiWordHexLength),
			want:   "",
		},
		{
			name:    "too-short payload",
			result:  "0x" + word("40"),
			wantErr: true,
		},
		{
			name:    "offset points past payload",
			result:  "0x" + word("ff") + strings.Repeat("0", abiWordHexLength),
			wantErr: true,
		},
		{
			name:    "length exceeds payload",
			result:  "0x" + word("40") + word("ff") + strings.Repeat("00", 16),
			wantErr: true,
		},
		{
			// Adversarial: offset is filled with the maximum signed-int64
			// value (after the leading zero in the 64-hex-char word).
			// IsInt64 passes; without the explicit `offset > len(stripped)`
			// guard, `offset * 2` would overflow to a negative number and
			// the subsequent slice would panic. With the guard, we return
			// a clean error.
			name: "max-int64 offset rejected before multiplication overflow",
			result: "0x" + "0" + strings.Repeat("7", 1) + strings.Repeat("f", 62) +
				strings.Repeat("00", 32),
			wantErr: true,
		},
		{
			// uint256 max as offset trips the IsInt64() branch (>2^63-1).
			name:    "uint256-max offset rejected (overflows int64)",
			result:  "0x" + strings.Repeat("f", 64) + strings.Repeat("00", 32),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDynamicString(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
