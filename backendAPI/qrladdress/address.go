// Package qrladdress implements the public QIP-55 address grammar used by
// the backend API. Storage remains Q-prefixed lowercase hex; public values
// are emitted in the canonical SHAKE256 checksum form.
package qrladdress

import (
	"crypto/sha3"
	"strings"
)

const (
	// HexLength is the number of hexadecimal characters in a 64-byte address.
	HexLength = 128
	// Length is the canonical Q-prefixed address length.
	Length = HexLength + 1
)

// IsValidAlias accepts the address spellings supported by read routes:
// Q/q or 0x/0X followed by exactly 128 hexadecimal characters. Uniform-case
// bodies are accepted. Mixed-case bodies must carry the QIP-55 checksum.
func IsValidAlias(address string) bool {
	body, ok := splitBody(address)
	if !ok || len(body) != HexLength || !isHex(body) {
		return false
	}
	return hasValidCase(body)
}

// IsValidCanonicalInput applies the Q-only input grammar used by contract
// write routes. Canonical output casing is available through Canonicalize.
func IsValidCanonicalInput(address string) bool {
	return strings.HasPrefix(address, "Q") && IsValidAlias(address)
}

// Canonicalize returns the uppercase-Q QIP-55 checksum form of any accepted
// alias. The bool is false for an invalid prefix, width, hex body, or checksum.
func Canonicalize(address string) (string, bool) {
	body, ok := splitBody(address)
	if !ok || len(body) != HexLength || !isHex(body) || !hasValidCase(body) {
		return "", false
	}
	lower := strings.ToLower(body)
	return "Q" + checksummedBody(lower), true
}

func splitBody(address string) (string, bool) {
	if len(address) > 0 && (address[0] == 'Q' || address[0] == 'q') {
		return address[1:], true
	}
	if len(address) > 1 && address[0] == '0' && (address[1] == 'x' || address[1] == 'X') {
		return address[2:], true
	}
	return "", false
}

func isHex(body string) bool {
	for _, c := range body {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func hasValidCase(body string) bool {
	hasLower := false
	hasUpper := false
	for _, c := range body {
		switch {
		case c >= 'a' && c <= 'f':
			hasLower = true
		case c >= 'A' && c <= 'F':
			hasUpper = true
		}
	}
	if !hasLower || !hasUpper {
		return true
	}
	lower := strings.ToLower(body)
	return body == checksummedBody(lower)
}

func checksummedBody(lower string) string {
	hash := make([]byte, HexLength/2)
	shake := sha3.NewSHAKE256()
	_, _ = shake.Write([]byte(lower))
	_, _ = shake.Read(hash)

	out := []byte(lower)
	for i, c := range out {
		if c < 'a' || c > 'f' {
			continue
		}
		nibble := hash[i/2]
		if i%2 == 0 {
			nibble >>= 4
		} else {
			nibble &= 0x0f
		}
		if nibble >= 8 {
			out[i] = c - ('a' - 'A')
		}
	}
	return string(out)
}
