package qns

import (
	"backendAPI/qrladdress"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// sha3HexAddress matches the SDK reverse label: Keccak-256 over the 128
// lowercase hexadecimal characters of a canonical address, without its prefix.
func sha3HexAddress(canonicalAddress string) [32]byte {
	return keccak256([]byte(strings.ToLower(canonicalAddress[1:])))
}

// reverseNodeFor matches the SDK node under addr.reverse for a canonical address.
func reverseNodeFor(canonicalAddress string) [32]byte {
	root := Namehash("addr.reverse")
	label := sha3HexAddress(canonicalAddress)
	var preimage [64]byte
	copy(preimage[:32], root[:])
	copy(preimage[32:], label[:])
	return keccak256(preimage[:])
}

// ReverseName returns a normalized primary name only after its forward record
// resolves to the same canonical address. Missing or invalid names stay empty.
func (c *Client) ReverseName(ctx context.Context, deployment Deployment, address string) (Resolution, error) {
	return c.ReverseNameWithLookup(ctx, deployment, address, func(ctx context.Context, name string) (Resolution, error) {
		return c.ResolveName(ctx, deployment, name)
	})
}

// ReverseNameWithLookup allows the route to share cached forward records.
// Resolver execution failures and invalid records are negative results.
// Registry and transport failures remain errors so a shared cache can retry.
func (c *Client) ReverseNameWithLookup(
	ctx context.Context,
	deployment Deployment,
	address string,
	lookup func(context.Context, string) (Resolution, error),
) (Resolution, error) {
	canonical, ok := qrladdress.Canonicalize(address)
	if !ok {
		return Resolution{}, ErrInvalidAddress
	}
	if c == nil || c.call == nil || lookup == nil || deployment.registryAddress == "" || deployment.expectedChainID == "" {
		return Resolution{}, ErrNotConfigured
	}
	node := reverseNodeFor(canonical)
	resolverResult, err := c.qrlCall(ctx, deployment.registryAddress, resolverCallData(node))
	if err != nil {
		return Resolution{}, fmt.Errorf("%w: %w", ErrRegistryLookup, err)
	}
	resolver, zero, err := decodeAddressWord(resolverResult)
	if err != nil {
		return Resolution{}, fmt.Errorf("%w: %w", ErrRegistryLookup, err)
	}
	if zero {
		return Resolution{Address: canonical, Missing: MissingResolver}, nil
	}

	missing := Resolution{Address: canonical, Missing: MissingName}
	nameResult, err := c.qrlCall(ctx, resolver, nameCallData(node))
	if err != nil {
		if errors.Is(err, ErrExecutionFailed) {
			return missing, nil
		}
		return Resolution{}, err
	}
	name, err := decodeName(nameResult)
	if err != nil {
		return missing, nil
	}
	normalized, err := NormalizeName(name)
	if err != nil {
		return missing, nil
	}
	forward, err := lookup(ctx, normalized)
	if err != nil {
		if errors.Is(err, ErrRegistryLookup) {
			return Resolution{}, err
		}
		if errors.Is(err, ErrExecutionFailed) || errors.Is(err, ErrMalformedRecord) {
			return missing, nil
		}
		return Resolution{}, err
	}
	if forward.Missing != MissingNone || forward.Address != canonical {
		return missing, nil
	}
	return Resolution{Name: normalized, Address: canonical}, nil
}

// decodeName reads a single QRVM64 ABI string: 64-byte offset and length words,
// followed by data padded to 64 bytes. Validate bounds before converting lengths.
func decodeName(value string) (string, error) {
	if value == "0x" {
		return "", nil
	}
	if !strings.HasPrefix(value, "0x") {
		return "", ErrMalformedResponse
	}
	data, err := hex.DecodeString(value[2:])
	const wordBytes = abiWordHex / 2
	if err != nil || len(data) < 2*wordBytes || len(data)%wordBytes != 0 {
		return "", ErrMalformedResponse
	}
	offset := new(big.Int).SetBytes(data[:wordBytes])
	if !offset.IsUint64() || offset.Uint64() != wordBytes {
		return "", ErrMalformedResponse
	}
	length := new(big.Int).SetBytes(data[wordBytes : 2*wordBytes])
	if !length.IsUint64() || length.Uint64() > uint64(len(data)-2*wordBytes) {
		return "", ErrMalformedResponse
	}
	end := 2*wordBytes + int(length.Uint64())
	padding := data[end:]
	if len(padding) >= wordBytes {
		return "", ErrMalformedResponse
	}
	for _, b := range padding {
		if b != 0 {
			return "", ErrMalformedResponse
		}
	}
	return string(data[2*wordBytes : end]), nil
}
