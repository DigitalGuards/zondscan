package rpc

import (
	"strings"
)

// Test fixtures: realistic Q-addresses that pass validation.IsValidAddress.
// `topic(addr)` left-pads the 20-byte address into a 32-byte indexed topic.
const (
	aliceAddr = "Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1"
	bobAddr   = "Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147"
	opAddr    = "Qa1b2c3d4e5f60708090a0b0c0d0e0f1011121314"
)

// topic builds an indexed-topic representation of a Q-prefix address:
// "0x" + 24 zero hex chars + 40-char lowercase hex.
func topic(addr string) string {
	stripped := strings.TrimPrefix(strings.ToLower(addr), "q")
	return "0x" + strings.Repeat("0", 24) + stripped
}

// word builds a 32-byte ABI-encoded uint256 word from a hex string (no 0x).
func word(hexStr string) string {
	if len(hexStr) > 64 {
		panic("word > 32 bytes")
	}
	return strings.Repeat("0", 64-len(hexStr)) + hexStr
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
