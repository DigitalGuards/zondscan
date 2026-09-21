package rpc

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestGetTokenBalanceWithCallerPropagatesAndDecodes(t *testing.T) {
	nodeErr := &RPCError{Code: -32000, Message: "historical state unavailable"}
	if _, err := getTokenBalanceWithCaller(
		opAddr,
		aliceAddr,
		func(string, string) (string, error) { return "", nodeErr },
	); !errors.Is(err, nodeErr) {
		t.Fatalf("token balance node error = %v, want %v", err, nodeErr)
	}
	wantCalldata := SIG_BALANCE + encodeAddressForABI(aliceAddr)
	balance, err := getTokenBalanceWithCaller(
		opAddr,
		aliceAddr,
		func(address, calldata string) (string, error) {
			if address != opAddr || calldata != wantCalldata {
				t.Fatalf("token balance call = %s %s", address, calldata)
			}
			return "0x" + word("2a"), nil
		},
	)
	if err != nil || balance != "42" {
		t.Fatalf("token balance = %q, %v", balance, err)
	}
}

// encodeDynamicResult builds a well-formed ABI dynamic-string qrl_call
// result: [offset=0x40 || length || data right-padded to 64B], 0x-prefixed.
func encodeDynamicResult(s string) string {
	raw := []byte(s)
	length := len(raw)
	padLen := (abiWordBytes - length%abiWordBytes) % abiWordBytes
	padded := make([]byte, length+padLen)
	copy(padded, raw)
	return "0x" + word("40") + word(new(big.Int).SetInt64(int64(length)).Text(16)) + hexEncode(padded)
}

// TestDecodeTokenSymbolRegression locks in the panic fix: the previous
// hand-rolled decoder assumed a fixed 0x20 offset and sliced
// `result[dataStart+64 : dataStart+64+int(length)*2]` with NO upper-bound
// check, so a response whose length word exceeds the payload panicked the
// syncer (slice bounds out of range). Routed through parseDynamicString it
// must return an error instead.
func TestDecodeTokenSymbolRegression(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			// Length word claims 255 bytes but only 16 bytes of data follow.
			// The old decoder sliced past the payload end and panicked here.
			name:    "length word exceeds payload returns error, not panic",
			result:  "0x" + word("40") + word("ff") + strings.Repeat("00", 16),
			wantErr: true,
		},
		{
			name:   "well-formed round trip",
			result: encodeDynamicResult("QTK"),
			want:   "QTK",
		},
		{
			name:   "well-formed 32-byte-boundary symbol",
			result: encodeDynamicResult("0123456789abcdef0123456789abcdef"),
			want:   "0123456789abcdef0123456789abcdef",
		},
		{
			// Preserved pre-fix behavior: an all-zero payload decodes to the
			// empty string without error (length word is zero).
			name:   "all-zero payload returns empty string",
			result: "0x" + strings.Repeat("0", 2*abiWordHexLength),
			want:   "",
		},
		{
			// Preserved pre-fix behavior: responses shorter than
			// "0x" + 256 hex chars are required up front.
			name:    "too-short response",
			result:  "0x" + word("40"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTokenSymbol(tt.result)
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

// TestDecodeTokenNameRegression locks in the panic fix for the name decoder.
// The old hand-rolled version bounds-checked its length read but parsed the
// offset with strconv.ParseInt and multiplied it by 2 unchecked: an offset of
// 0x4000000000000000 (2^62) passes ParseInt, overflows int64 on `offset*2`
// into a negative startPos that slipped past the `startPos+64 <= len` guard,
// and the subsequent slice panicked. parseDynamicString rejects the offset
// before the multiplication.
func TestDecodeTokenNameRegression(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			// Offset word 2^62 (the overflow panic vector). The trailing
			// data is crafted so the fixed-string fallback also fails
			// (odd hex-char count after zero trimming), proving the error
			// path is reached instead of a panic or a garbage decode.
			name:    "overflowing offset word returns error, not panic",
			result:  "0x" + word("4000000000000000") + "5" + strings.Repeat("0", abiWordHexLength-1),
			wantErr: true,
		},
		{
			name:   "well-formed round trip",
			result: encodeDynamicResult("Quantum Token"),
			want:   "Quantum Token",
		},
		{
			// Format 2 (fixed string) fallback preserved: the entire
			// response is the hex-encoded string, no offset/length words.
			name:   "fixed-string fallback still decodes",
			result: "0x" + hexEncode([]byte("Legacy")) + strings.Repeat("0", 52),
			want:   "Legacy",
		},
		{
			name:    "empty result returns error",
			result:  "0x",
			wantErr: true,
		},
		{
			name:    "all-zero result returns error",
			result:  "0x" + strings.Repeat("0", 2*abiWordHexLength),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTokenName(tt.result)
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

func TestDecodeTokenDecimals(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		want    uint8
		wantErr bool
	}{
		{name: "zero", result: "0x" + word("0"), want: 0},
		{name: "common value", result: "0x" + word("12"), want: 18},
		{name: "uint8 maximum", result: "0x" + word("ff"), want: 255},
		{name: "no prefix", result: word("8"), want: 8},
		{name: "uint8 overflow", result: "0x" + word("100"), wantErr: true},
		{
			name:    "nonzero high bytes",
			result:  "0x1" + strings.Repeat("0", abiWordHexLength-1),
			wantErr: true,
		},
		{
			name:    "legacy 32-byte word",
			result:  "0x" + strings.Repeat("0", uint256HexLength-2) + "12",
			wantErr: true,
		},
		{name: "empty", result: "0x", wantErr: true},
		{name: "non-hex", result: "0x" + strings.Repeat("0", abiWordHexLength-1) + "z", wantErr: true},
		{name: "two words", result: "0x" + word("12") + word("0"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTokenDecimals(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDecodeTokenTotalSupply(t *testing.T) {
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{name: "zero", result: "0x" + word("0"), want: "0"},
		{name: "ordinary supply", result: "0x" + word("3b9aca00"), want: "1000000000"},
		{
			name:   "uint256 maximum",
			result: "0x" + word(strings.Repeat("f", uint256HexLength)),
			want:   maxUint256.String(),
		},
		{name: "truncated word", result: "0x" + word("1")[:abiWordHexLength-1], wantErr: true},
		{name: "trailing word", result: "0x" + word("1") + word("2"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTokenTotalSupply(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDecodeTokenBalance(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{name: "zero", result: "0x" + word("0"), want: "0"},
		{name: "balance", result: "0x" + word("2a"), want: "42"},
		{name: "empty result", result: "", wantErr: true},
		{
			name:    "noncanonical high bytes",
			result:  "0x" + strings.Repeat("f", uint256HexLength) + strings.Repeat("0", uint256HexLength),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTokenBalance(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}
