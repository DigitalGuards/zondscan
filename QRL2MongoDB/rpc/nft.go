package rpc

import (
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"
)

// GetERC721Owner queries `ownerOf(uint256)` on an ERC-721 contract and returns
// the current owner's Q-prefix address.
//
// Three-way error contract (mirrors SupportsInterface):
//   - Contract revert (burned or never-minted id): returns ("", nil), callers
//     treat as "no current owner" and delete any stale balance row.
//   - Transport error: returns ("", err), callers preserve existing state to
//     uphold the C5 promote-only invariant.
//   - Zero address / empty word: returns ("", nil), same "no owner" semantics.
//
// TODO(scale): one RPC call per ERC-721 transfer. Testnet handles it; mainnet
// at high NFT volume would want an event-delta accumulator with periodic
// reconciliation instead.
func GetERC721Owner(contractAddress string, tokenID *big.Int) (string, error) {
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return "", fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_OWNER_OF + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		// JSON-RPC error => contract reverted (typical for burned/unminted ids).
		// Anything else (transport, marshalling) is transient; propagate so the
		// caller can preserve existing state.
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	return parseAddressFromWord(result)
}

// GetContractURI queries `contractURI()` on a contract (OpenSea convention
// for collection-level NFT metadata) and returns the raw URI string.
//
// Three-way error contract, mirrors SupportsInterface / GetERC721Owner:
//   - Contract revert (the contract doesn't implement contractURI, which is
//     true of most collections): returns ("", nil) so the caller can record
//     "no collection metadata" without treating it as a fetch failure.
//   - Transport error: returns ("", err); callers preserve existing state
//     (C5 promote-only invariant).
//   - Empty / malformed return: returns ("", nil), same "no URI" semantic.
//
// The returned URI is whatever the contract emitted, no IPFS gateway
// resolution happens here, the metadata service does that.
func GetContractURI(contractAddress string) (string, error) {
	result, callErr := CallContractMethod(contractAddress, SIG_CONTRACT_URI)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	uri, decErr := parseDynamicString(result)
	if decErr != nil {
		// Malformed return word, treat as "no URI" rather than failing the
		// caller; we never want a single misbehaving contract to throw off
		// classification of every contract that comes after.
		return "", nil
	}
	return uri, nil
}

// GetTokenURI queries `tokenURI(uint256)` on an ERC-721 contract and returns
// the raw URI string. Same three-way error contract as the rest of the
// tokens helpers:
//   - Contract revert (id never minted, or contract doesn't implement
//     ERC-721 Metadata): returns ("", nil) so the caller records "no URI"
//     without treating it as a fetch failure.
//   - Transport error: returns ("", err); callers preserve existing state.
//   - Malformed return: returns ("", nil), same "no URI" semantic.
//
// The URI is returned exactly as the contract emitted it; the metadata
// fetcher service handles IPFS gateway resolution + JSON parsing.
func GetTokenURI(contractAddress string, tokenID *big.Int) (string, error) {
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return "", fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_TOKEN_URI + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	uri, decErr := parseDynamicString(result)
	if decErr != nil {
		// Same defensive posture as GetContractURI: a malformed return
		// from one contract must not knock subsequent fetches off course.
		return "", nil
	}
	return uri, nil
}

// GetERC1155URI queries `uri(uint256)` on an ERC-1155 contract.
//
// ERC-1155 spec note: the returned URI may contain the substring `{id}`,
// which clients must substitute with the lowercase 64-char hex form of the
// tokenID (so id 42 becomes "000...02a"). This function does that
// substitution before returning, so the caller can pass the result
// straight to the metadata fetcher without knowing the spec quirk.
//
// Same three-way error contract as GetTokenURI.
func GetERC1155URI(contractAddress string, tokenID *big.Int) (string, error) {
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return "", fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_URI + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	uri, decErr := parseDynamicString(result)
	if decErr != nil {
		return "", nil
	}
	return substituteERC1155IDTemplate(uri, tokenID), nil
}

// substituteERC1155IDTemplate replaces the `{id}` placeholder per the
// ERC-1155 spec: "Clients MUST replace any occurrences of the substring
// `{id}` in the URI with the actual token ID, in lowercase hexadecimal
// (with no 0x prefix) and leading-zero-padded to 64 hex characters".
//
// Empty input or no placeholder => return unchanged.
func substituteERC1155IDTemplate(uri string, tokenID *big.Int) string {
	if uri == "" || !strings.Contains(uri, "{id}") {
		return uri
	}
	h := tokenID.Text(16)
	if len(h) > 64 {
		// Shouldn't happen for any realistic tokenID (uint256 max is 64
		// hex), but truncate-from-left rather than panic if it does.
		h = h[len(h)-64:]
	}
	padded := strings.Repeat("0", 64-len(h)) + h
	return strings.ReplaceAll(uri, "{id}", padded)
}

// GetERC1155Balance queries `balanceOf(address,uint256)` on an ERC-1155
// contract and returns the holder's current per-id balance.
//
// Three-way error contract:
//   - Contract revert: returns (big.NewInt(0), nil), treated as "no balance"
//     (sparse storage; caller deletes the row).
//   - Transport error: returns (nil, err), caller preserves existing state.
//   - Empty / zero return: returns (big.NewInt(0), nil), same as revert.
//
// TODO(scale): one RPC call per (from, to) side of an ERC-1155 transfer.
// Same accumulator/reconciliation concern as GetERC721Owner above.
func GetERC1155Balance(contractAddress, holderAddress string, tokenID *big.Int) (*big.Int, error) {
	if !validation.IsValidAddress(holderAddress) {
		return nil, fmt.Errorf("invalid holder address: %s", holderAddress)
	}
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return nil, fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_BALANCE_OF_1155 + encodeAddressForABI(holderAddress) + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return big.NewInt(0), nil
		}
		return nil, callErr
	}
	return parseUint256FromWord(result)
}
