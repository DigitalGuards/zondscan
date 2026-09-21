// Package qns implements the fail-closed QNS forward-resolution protocol
// used by the Zondscan backend.
package qns

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/sha3"
)

const (
	// MaxNameBytes bounds public route work while covering the conventional
	// DNS wire-format limit. The SDK normalization rules apply unchanged
	// within this bound.
	MaxNameBytes = 255
	abiWordHex   = 128

	resolverSelector = "0178b8bf"
	addrSelector     = "3b3b57de"
)

var ErrInvalidName = errors.New("invalid QNS name")

// NormalizeName matches @qns/sdk's conservative normalization profile and
// additionally requires a resolvable name below the supported .qrl TLD.
// ASCII uppercase folds to lowercase. Every label then accepts only a-z,
// 0-9, and hyphen, with the SDK's reserved double-hyphen rule at positions
// three and four.
func NormalizeName(name string) (string, error) {
	if name == "" || len(name) > MaxNameBytes {
		return "", ErrInvalidName
	}

	lowered := make([]byte, len(name))
	for i, c := range []byte(name) {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lowered[i] = c
	}

	labels := strings.Split(string(lowered), ".")
	if len(labels) < 2 || labels[len(labels)-1] != "qrl" {
		return "", fmt.Errorf("%w: expected a name below .qrl", ErrInvalidName)
	}
	for _, label := range labels {
		if label == "" {
			return "", fmt.Errorf("%w: empty label", ErrInvalidName)
		}
		for _, c := range []byte(label) {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return "", fmt.Errorf("%w: unsupported label character", ErrInvalidName)
			}
		}
		if len(label) >= 4 && label[2] == '-' && label[3] == '-' {
			return "", fmt.Errorf("%w: reserved double-hyphen pattern", ErrInvalidName)
		}
	}
	return strings.Join(labels, "."), nil
}

// Namehash implements EIP-137 over an already-normalized name. QNS uses
// Keccak-256 here, matching @qns/sdk and the registry contract.
func Namehash(name string) [32]byte {
	var node [32]byte
	if name == "" {
		return node
	}
	labels := strings.Split(name, ".")
	for i := len(labels) - 1; i >= 0; i-- {
		labelHash := keccak256([]byte(labels[i]))
		preimage := make([]byte, 0, len(node)+len(labelHash))
		preimage = append(preimage, node[:]...)
		preimage = append(preimage, labelHash[:]...)
		node = keccak256(preimage)
	}
	return node
}

func keccak256(data []byte) [32]byte {
	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write(data)
	sum := h.Sum(nil)
	var out [32]byte
	copy(out[:], sum)
	return out
}

func resolverCallData(node [32]byte) string {
	return encodeBytes32Call(resolverSelector, node)
}

func addrCallData(node [32]byte) string {
	return encodeBytes32Call(addrSelector, node)
}

// QRVM64 ABI words are 64 bytes. A bytes32 argument occupies the high half
// of one word and is right-padded by 32 zero bytes.
func encodeBytes32Call(selector string, node [32]byte) string {
	return "0x" + selector + hex.EncodeToString(node[:]) + strings.Repeat("0", abiWordHex/2)
}
