package qns

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidNode reports a node that is not 0x followed by 64 hex characters.
var ErrInvalidNode = errors.New("invalid QNS node")

// NodeRecord describes one registry node. Owner and Resolver are canonical
// QIP-55 addresses or empty for the zero address. Name is set only when the
// resolver's name record normalizes and hashes back to Node.
type NodeRecord struct {
	Node     string
	Owner    string
	Resolver string
	Name     string
}

// ParseNode reads a 32-byte node written as 0x followed by 64 hex characters.
func ParseNode(value string) ([32]byte, error) {
	var node [32]byte
	if len(value) != 66 || (!strings.HasPrefix(value, "0x") && !strings.HasPrefix(value, "0X")) {
		return node, ErrInvalidNode
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil {
		return node, ErrInvalidNode
	}
	copy(node[:], decoded)
	return node, nil
}

// LookupNode reads owner(bytes32) and resolver(bytes32) from the checked
// registry, then the resolver's name(bytes32) record. A published name is
// returned only when namehash(normalized name) equals the node. Resolver
// execution failures and invalid name records leave the name empty; registry
// and transport failures remain errors so a shared cache can retry.
func (c *Client) LookupNode(ctx context.Context, deployment Deployment, node [32]byte) (NodeRecord, error) {
	if c == nil || c.call == nil || deployment.registryAddress == "" || deployment.expectedChainID == "" {
		return NodeRecord{}, ErrNotConfigured
	}
	record := NodeRecord{Node: "0x" + hex.EncodeToString(node[:])}

	owner, err := c.registryAddressWord(ctx, deployment, ownerCallData(node))
	if err != nil {
		return NodeRecord{}, err
	}
	if owner == "" {
		return record, nil
	}
	record.Owner = owner

	resolver, err := c.registryAddressWord(ctx, deployment, resolverCallData(node))
	if err != nil {
		return NodeRecord{}, err
	}
	if resolver == "" {
		return record, nil
	}
	record.Resolver = resolver

	nameResult, err := c.qrlCall(ctx, resolver, nameCallData(node))
	if err != nil {
		if errors.Is(err, ErrExecutionFailed) {
			return record, nil
		}
		return NodeRecord{}, err
	}
	name, err := decodeName(nameResult)
	if err != nil || name == "" {
		return record, nil
	}
	normalized, err := NormalizeName(name)
	if err != nil || Namehash(normalized) != node {
		return record, nil
	}
	record.Name = normalized
	return record, nil
}

// registryAddressWord performs one registry read that returns an address
// word. The zero address returns an empty string.
func (c *Client) registryAddressWord(ctx context.Context, deployment Deployment, data string) (string, error) {
	result, err := c.qrlCall(ctx, deployment.registryAddress, data)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRegistryLookup, err)
	}
	address, zero, err := decodeAddressWord(result)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRegistryLookup, err)
	}
	if zero {
		return "", nil
	}
	return address, nil
}
