package rpc

import (
	"strings"
)

// Test fixtures: native QIP-55 addresses that pass validation.IsValidAddress.
// A full-width address occupies one 64-byte indexed topic without padding.
const (
	aliceAddr = "Q" +
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
	bobAddr = "Q" +
		"ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100" +
		"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	opAddr = "Q" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
		"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// topic builds an indexed-topic representation of a Q-prefix address:
// "0x" + the complete 128-character lowercase address body.
func topic(addr string) string {
	stripped := strings.TrimPrefix(strings.ToLower(addr), "q")
	return "0x" + stripped
}

// word builds a uint256 value right-aligned in a 64-byte ABI word.
func word(hexStr string) string {
	if len(hexStr) > uint256HexLength {
		panic("uint256 > 32 bytes")
	}
	return strings.Repeat("0", abiWordHexLength-len(hexStr)) + hexStr
}

// hexEncode is an inline lower-case hex.EncodeToString without importing.
func hexEncode(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexd[v>>4]
		out[i*2+1] = hexd[v&0x0f]
	}
	return string(out)
}
