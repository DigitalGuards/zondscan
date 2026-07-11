package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"
)

// TrimLeftZeros trims leading zeros from hex string
func TrimLeftZeros(hex string) string {
	for i := 0; i < len(hex); i++ {
		if hex[i] != '0' {
			return hex[i:]
		}
	}
	return "0"
}

// maxBatchArrayLen caps the per-array element count we'll decode from a
// TransferBatch log. Real-world batches are <100; this guards against an
// adversarial payload claiming a huge array to OOM the decoder.
const maxBatchArrayLen = 10000

// readUint64FromWord parses one 32-byte ABI word at hex position `posHex`
// and returns its value as uint64. Returns an error if the value exceeds
// uint64 range (offsets in real logs are tens to hundreds of bytes).
func readUint64FromWord(data string, posHex uint64) (uint64, error) {
	if posHex+64 > uint64(len(data)) {
		return 0, fmt.Errorf("data too short at hex offset %d", posHex)
	}
	word := new(big.Int)
	if _, ok := word.SetString(data[posHex:posHex+64], 16); !ok {
		return 0, fmt.Errorf("failed to parse word at hex offset %d", posHex)
	}
	if !word.IsUint64() {
		return 0, fmt.Errorf("word value exceeds uint64 at hex offset %d", posHex)
	}
	return word.Uint64(), nil
}

// decodeUint256Array reads an ABI-encoded uint256[] starting at hex position
// `startHex` of `data`. The encoding is `[length(32B) || elements(32B each)]`.
func decodeUint256Array(data string, startHex uint64) ([]*big.Int, error) {
	length, err := readUint64FromWord(data, startHex)
	if err != nil {
		return nil, err
	}
	if length > maxBatchArrayLen {
		return nil, fmt.Errorf("array length %d exceeds cap %d", length, maxBatchArrayLen)
	}
	elemStart := startHex + 64
	end := elemStart + length*64
	if end > uint64(len(data)) {
		return nil, fmt.Errorf("data too short for %d elements", length)
	}
	out := make([]*big.Int, length)
	for i := uint64(0); i < length; i++ {
		pos := elemStart + i*64
		item := new(big.Int)
		if _, ok := item.SetString(data[pos:pos+64], 16); !ok {
			return nil, fmt.Errorf("failed to parse element %d", i)
		}
		out[i] = item
	}
	return out, nil
}

// encodeAddressForABI strips the Q/0x prefix from `addr` and left-pads it to a
// 32-byte word for ABI encoding. Caller is responsible for passing a validated
// address; the upstream `validation.IsValidAddress` check is enforced where
// callers source addresses from log topics or user input.
func encodeAddressForABI(addr string) string {
	raw := strings.ToLower(validation.StripAddressPrefix(addr))
	if len(raw) > 64 {
		raw = raw[len(raw)-64:]
	}
	return strings.Repeat("0", 64-len(raw)) + raw
}

// encodeUint256ForABI left-pads `v` to a 32-byte uint256 word.
// Negative values are rejected (uint256 has no sign).
func encodeUint256ForABI(v *big.Int) (string, error) {
	if v == nil {
		return "", fmt.Errorf("uint256 cannot be nil")
	}
	if v.Sign() < 0 {
		return "", fmt.Errorf("uint256 cannot be negative: %s", v.String())
	}
	h := v.Text(16)
	if len(h) > 64 {
		return "", fmt.Errorf("uint256 exceeds 32 bytes: %s", v.String())
	}
	return strings.Repeat("0", 64-len(h)) + h, nil
}

// parseAddressFromWord decodes the rightmost 32-byte word of `result` into a
// canonical Q-prefix lowercase address. Returns ("", nil) if the word is the
// zero address, callers interpret that as "no owner" (sparse storage).
func parseAddressFromWord(result string) (string, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) < 64 {
		return "", fmt.Errorf("address word too short: %d hex chars", len(stripped))
	}
	word := stripped[len(stripped)-64:]
	// All-zero word is the zero address: treat as "no owner".
	if strings.TrimLeft(word, "0") == "" {
		return "", nil
	}
	addrHex := word[len(word)-40:]
	// The high 12 bytes (24 hex chars) must be zero for a valid 20-byte address.
	if strings.TrimLeft(word[:24], "0") != "" {
		return "", fmt.Errorf("address word has nonzero high bytes: %s", word)
	}
	addr := "Q" + strings.ToLower(addrHex)
	if !validation.IsValidAddress(addr) {
		return "", fmt.Errorf("invalid address derived from word: %s", addr)
	}
	return addr, nil
}

// parseUint256FromWord decodes the rightmost 32-byte word of `result` into a
// *big.Int. Returns nil + error when the word is shorter than 32 bytes or
// fails hex parsing.
func parseUint256FromWord(result string) (*big.Int, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) < 64 {
		return nil, fmt.Errorf("uint256 word too short: %d hex chars", len(stripped))
	}
	word := stripped[len(stripped)-64:]
	v := new(big.Int)
	if _, ok := v.SetString(word, 16); !ok {
		return nil, fmt.Errorf("failed to parse uint256 from word: %s", word)
	}
	return v, nil
}

// parseDynamicString decodes the ABI-encoded `bytes`/`string` return value of
// a no-argument view function (e.g. name(), symbol(), contractURI()).
//
// The ABI layout for a single dynamic return value is:
//
//	[ offset (32B) || length (32B) || data (length bytes, right-padded to 32B) ]
//
// `result` is the raw `qrl_call` result with or without the "0x" prefix.
// Empty or all-zero results return ("", nil), the caller treats that as
// "method returned empty" which is distinct from a transport / decode error.
//
// Defensive against malformed payloads: every offset / length read is
// bounds-checked against the available hex chars, so a truncated response
// returns a clean error instead of panicking.
func parseDynamicString(result string) (string, error) {
	stripped := strings.TrimPrefix(result, "0x")

	// Empty or all-zero is treated as "method exists but returned empty
	// string"; not an error so callers can record `metadataURI=""` as a
	// successful probe (no URI set on chain).
	if len(stripped) == 0 || strings.TrimLeft(stripped, "0") == "" {
		return "", nil
	}

	// Minimum payload: 32 bytes offset + 32 bytes length = 128 hex chars.
	if len(stripped) < 128 {
		return "", fmt.Errorf("dynamic string payload too short: %d hex chars", len(stripped))
	}

	// ABI words are 256 bits; strconv.ParseInt caps at 64. Real offsets are
	// tiny (start at 0x20 = 32 bytes for one dynamic return), but an
	// attacker-crafted return value could fill the full 32-byte word to try
	// to crash the decoder. Parse as big.Int, reject negatives + anything
	// outside int64, then bound against the payload size BEFORE the *2
	// multiplication so an offset near math.MaxInt64 can't overflow
	// startPos into a negative number that would silently bypass the
	// subsequent slice-bounds check and panic on `stripped[startPos:...]`.
	offsetInt := new(big.Int)
	if _, ok := offsetInt.SetString(stripped[:64], 16); !ok {
		return "", fmt.Errorf("invalid offset word")
	}
	if !offsetInt.IsInt64() || offsetInt.Sign() < 0 {
		return "", fmt.Errorf("offset out of int64 range")
	}
	offset := offsetInt.Int64()
	// `offset` is a byte index into the payload; `len(stripped)` is hex
	// chars (2x byte count). The check `offset > len(stripped)` is a
	// generous upper bound that's still tight enough to prevent the
	// multiplication overflow, the next bounds check below handles the
	// precise condition.
	if offset > int64(len(stripped)) {
		return "", fmt.Errorf("offset out of bounds: offset=%d payload=%d", offset, len(stripped)/2)
	}
	startPos := offset * 2
	if startPos+64 > int64(len(stripped)) {
		return "", fmt.Errorf("string length word out of bounds: offset=%d payload=%d", offset, len(stripped)/2)
	}

	lengthInt := new(big.Int)
	if _, ok := lengthInt.SetString(stripped[startPos:startPos+64], 16); !ok {
		return "", fmt.Errorf("invalid length word")
	}
	if !lengthInt.IsInt64() || lengthInt.Sign() < 0 {
		return "", fmt.Errorf("length out of int64 range")
	}
	length := lengthInt.Int64()
	// Cap at a sane upper bound; nothing legitimate is going to return a
	// multi-MB string from a view call, and an attacker could otherwise
	// force a huge allocation.
	const maxDynamicStringBytes = 1 << 20 // 1 MiB
	if length > maxDynamicStringBytes {
		return "", fmt.Errorf("dynamic string length %d exceeds cap %d", length, maxDynamicStringBytes)
	}
	if startPos+64+length*2 > int64(len(stripped)) {
		return "", fmt.Errorf("string data out of bounds: declared length=%d available=%d", length, (int64(len(stripped))-startPos-64)/2)
	}

	dataHex := stripped[startPos+64 : startPos+64+length*2]
	bytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	return string(bytes), nil
}
