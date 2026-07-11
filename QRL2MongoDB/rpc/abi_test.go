package rpc

import (
	"math/big"
	"strings"
	"testing"
)

func TestEncodeAddressForABI(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "Q-prefix lowercase",
			addr: aliceAddr,
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
		{
			name: "0x-prefix uppercase canonicalised to lowercase",
			addr: "0x6153D37FA4DA7193E6219DCBD2BBE62FA12905B1",
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
		{
			name: "no prefix",
			addr: "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeAddressForABI(tt.addr)
			if got != tt.want {
				t.Errorf("encodeAddressForABI(%s)\n got %s\nwant %s", tt.addr, got, tt.want)
			}
			if len(got) != 64 {
				t.Errorf("encoded length = %d, want 64", len(got))
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
			want: strings.Repeat("0", 64),
		},
		{
			name: "42",
			v:    big.NewInt(42),
			want: strings.Repeat("0", 62) + "2a",
		},
		{
			name: "uint256 max",
			v:    maxUint256,
			want: strings.Repeat("f", 64),
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
			name:     "happy path: alice's address padded",
			result:   "0x" + strings.Repeat("0", 24) + aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:     "zero address returns empty (no owner)",
			result:   "0x" + strings.Repeat("0", 64),
			wantAddr: "",
		},
		{
			name:     "no 0x prefix accepted",
			result:   strings.Repeat("0", 24) + aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:    "too-short response",
			result:  "0x" + aliceRaw,
			wantErr: true,
		},
		{
			name:    "nonzero high bytes (malformed)",
			result:  "0x" + "ff" + strings.Repeat("0", 22) + aliceRaw,
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
			result: "0x" + strings.Repeat("0", 64),
			want:   "0",
		},
		{
			name:   "uint256 max",
			result: "0x" + maxHex,
			want:   maxUint256.String(),
		},
		{
			name:    "too short",
			result:  "0x" + word("64")[:32],
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

func TestParseDynamicString(t *testing.T) {
	// Helper: build an ABI-encoded dynamic string from a Go string.
	// Layout: [offset=0x20 || length || data right-padded to 32B].
	encode := func(s string) string {
		raw := []byte(s)
		length := len(raw)
		// Right-pad to next 32-byte boundary.
		padLen := (32 - length%32) % 32
		padded := make([]byte, length+padLen)
		copy(padded, raw)
		offsetWord := word("20")
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
			result: "0x" + word("20") + word("0"),
			want:   "",
		},
		{
			name:   "all-zero payload returns empty (no error)",
			result: "0x" + strings.Repeat("0", 128),
			want:   "",
		},
		{
			name:    "too-short payload",
			result:  "0x" + word("20"),
			wantErr: true,
		},
		{
			name:    "offset points past payload",
			result:  "0x" + word("ff") + strings.Repeat("0", 64),
			wantErr: true,
		},
		{
			name:    "length exceeds payload",
			result:  "0x" + word("20") + word("ff") + strings.Repeat("00", 16),
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
