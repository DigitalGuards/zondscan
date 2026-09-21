package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"
)

// maxBatchArrayLen caps the per-array element count we'll decode from a
// TransferBatch log. Real-world batches are <100; this guards against an
// adversarial payload claiming a huge array to OOM the decoder.
const (
	abiWordBytes     = 64
	abiWordHexLength = abiWordBytes * 2
	uint256HexLength = 64
	maxBatchArrayLen = 10000
)

func checkedUint64Add(left, right uint64) (uint64, error) {
	if left > ^uint64(0)-right {
		return 0, fmt.Errorf("uint64 addition overflows: %d + %d", left, right)
	}
	return left + right, nil
}

func checkedUint64Multiply(left, right uint64) (uint64, error) {
	if left != 0 && right > ^uint64(0)/left {
		return 0, fmt.Errorf("uint64 multiplication overflows: %d * %d", left, right)
	}
	return left * right, nil
}

func encodedUint256ArrayEndBytes(startBytes uint64, length uint64) (uint64, error) {
	payloadBytes, err := checkedUint64Multiply(length, uint64(abiWordBytes))
	if err != nil {
		return 0, err
	}
	startWithLength, err := checkedUint64Add(startBytes, uint64(abiWordBytes))
	if err != nil {
		return 0, err
	}
	return checkedUint64Add(startWithLength, payloadBytes)
}

// readUint64FromWord parses one 64-byte ABI word at hex position `posHex`
// and returns its value as uint64. Returns an error if the value exceeds
// uint64 range (offsets in real logs are tens to hundreds of bytes).
func readUint64FromWord(data string, posHex uint64) (uint64, error) {
	dataLength := uint64(len(data))
	wordLength := uint64(abiWordHexLength)
	if posHex > dataLength || wordLength > dataLength-posHex {
		return 0, fmt.Errorf("data too short at hex offset %d", posHex)
	}
	end := posHex + wordLength
	word, err := parseUint256HexWord(data[posHex:end])
	if err != nil {
		return 0, fmt.Errorf("failed to parse word at hex offset %d: %w", posHex, err)
	}
	if !word.IsUint64() {
		return 0, fmt.Errorf("word value exceeds uint64 at hex offset %d", posHex)
	}
	return word.Uint64(), nil
}

// decodeUint256Array reads an ABI-encoded uint256[] starting at hex position
// `startHex` of `data`. Each length and element occupies a 64-byte ABI word.
func decodeUint256Array(data string, startHex uint64) ([]*big.Int, error) {
	length, err := readUint64FromWord(data, startHex)
	if err != nil {
		return nil, err
	}
	if length > maxBatchArrayLen {
		return nil, fmt.Errorf("array length %d exceeds cap %d", length, maxBatchArrayLen)
	}
	wordLength := uint64(abiWordHexLength)
	elemStart, err := checkedUint64Add(startHex, wordLength)
	if err != nil {
		return nil, fmt.Errorf("array start overflows at hex offset %d: %w", startHex, err)
	}
	payloadLength, err := checkedUint64Multiply(length, wordLength)
	if err != nil {
		return nil, fmt.Errorf("array length %d overflows encoded size: %w", length, err)
	}
	dataLength := uint64(len(data))
	if elemStart > dataLength || payloadLength > dataLength-elemStart {
		return nil, fmt.Errorf("data too short for %d elements", length)
	}
	out := make([]*big.Int, length)
	for i := uint64(0); i < length; i++ {
		pos := elemStart + i*wordLength
		item, err := parseUint256HexWord(data[pos : pos+wordLength])
		if err != nil {
			return nil, fmt.Errorf("failed to parse element %d: %w", i, err)
		}
		out[i] = item
	}
	return out, nil
}

// encodeAddressForABI strips the Q/0x prefix from `addr`. A QIP-55 address is
// exactly one 64-byte ABI word, so no padding or truncation is required. The
// function rejects any body with the wrong width or non-hexadecimal content.
func encodeAddressForABI(addr string) string {
	raw := strings.ToLower(validation.StripAddressPrefix(addr))
	if len(raw) != abiWordHexLength || !validation.IsValidAddress("Q"+raw) {
		return ""
	}
	return raw
}

// encodeUint256ForABI left-pads `v` into the low 32 bytes of a 64-byte ABI
// word. Negative values and values wider than uint256 are rejected.
func encodeUint256ForABI(v *big.Int) (string, error) {
	if v == nil {
		return "", fmt.Errorf("uint256 cannot be nil")
	}
	if v.Sign() < 0 {
		return "", fmt.Errorf("uint256 cannot be negative: %s", v.String())
	}
	h := v.Text(16)
	if len(h) > uint256HexLength {
		return "", fmt.Errorf("uint256 exceeds 32 bytes: %s", v.String())
	}
	return strings.Repeat("0", abiWordHexLength-len(h)) + h, nil
}

// parseAddressFromWord decodes one exact 64-byte word of `result` into a
// canonical Q-prefix lowercase address. Returns ("", nil) if the word is the
// zero address, callers interpret that as "no owner" (sparse storage).
func parseAddressFromWord(result string) (string, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) != abiWordHexLength {
		return "", fmt.Errorf("address result has %d hex chars, want %d", len(stripped), abiWordHexLength)
	}
	word := stripped
	// All-zero word is the zero address: treat as "no owner".
	if strings.TrimLeft(word, "0") == "" {
		return "", nil
	}
	addr := "Q" + strings.ToLower(word)
	if !validation.IsValidAddress(addr) {
		return "", fmt.Errorf("invalid address derived from word: %s", addr)
	}
	return addr, nil
}

// parseUint256FromWord decodes the low 32 bytes of one exact 64-byte ABI
// word. The high 32 bytes must be zero for a canonical uint256 value.
func parseUint256FromWord(result string) (*big.Int, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) != abiWordHexLength {
		return nil, fmt.Errorf("uint256 result has %d hex chars, want %d", len(stripped), abiWordHexLength)
	}
	return parseUint256HexWord(stripped)
}

func parseUint256HexWord(word string) (*big.Int, error) {
	if len(word) != abiWordHexLength {
		return nil, fmt.Errorf("uint256 word has %d hex chars, want %d", len(word), abiWordHexLength)
	}
	if strings.TrimLeft(word[:abiWordHexLength-uint256HexLength], "0") != "" {
		return nil, fmt.Errorf("uint256 word has nonzero high bytes: %s", word)
	}
	v := new(big.Int)
	if _, ok := v.SetString(word[abiWordHexLength-uint256HexLength:], 16); !ok {
		return nil, fmt.Errorf("failed to parse uint256 from word: %s", word)
	}
	return v, nil
}

// parseDynamicString decodes the ABI-encoded `bytes`/`string` return value of
// a no-argument view function (e.g. name(), symbol(), contractURI()).
//
// The ABI layout for a single dynamic return value is:
//
//	[ offset (64B) || length (64B) || data (length bytes, right-padded to 64B) ]
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

	// Minimum payload: 64 bytes offset + 64 bytes length = 256 hex chars.
	if len(stripped) < 2*abiWordHexLength {
		return "", fmt.Errorf("dynamic string payload too short: %d hex chars", len(stripped))
	}

	// Offsets are uint256 values carried in the low half of a 64-byte ABI word.
	// Real offsets are tiny (start at 0x40 = 64 bytes for one dynamic return),
	// but an attacker-crafted return value could still try to crash the decoder.
	// Reject non-canonical high bytes and anything outside int64, then bound
	// against the payload size BEFORE the *2
	// multiplication so an offset near math.MaxInt64 can't overflow
	// startPos into a negative number that would silently bypass the
	// subsequent slice-bounds check and panic on `stripped[startPos:...]`.
	offsetInt, err := parseUint256HexWord(stripped[:abiWordHexLength])
	if err != nil {
		return "", fmt.Errorf("invalid offset word: %w", err)
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
	if startPos+abiWordHexLength > int64(len(stripped)) {
		return "", fmt.Errorf("string length word out of bounds: offset=%d payload=%d", offset, len(stripped)/2)
	}

	lengthInt, err := parseUint256HexWord(stripped[startPos : startPos+abiWordHexLength])
	if err != nil {
		return "", fmt.Errorf("invalid length word: %w", err)
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
	if startPos+abiWordHexLength+length*2 > int64(len(stripped)) {
		return "", fmt.Errorf("string data out of bounds: declared length=%d available=%d", length, (int64(len(stripped))-startPos-abiWordHexLength)/2)
	}

	dataHex := stripped[startPos+abiWordHexLength : startPos+abiWordHexLength+length*2]
	bytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	return string(bytes), nil
}
