package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"

	"go.uber.org/zap"
)

// GetTokenName retrieves the name of an ERC20 token
func GetTokenName(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_NAME)
	if err != nil {
		return "", err
	}
	return decodeTokenName(result)
}

// decodeTokenName decodes a name() qrl_call result. Split from GetTokenName
// so the decode path is testable without a live node.
//
// Handles two response formats:
//
//   - Format 1: dynamic string (most common), decoded through the hardened
//     parseDynamicString (offset word + length word + data, every read
//     bounds-checked). This replaced a hand-rolled decoder whose unchecked
//     offset arithmetic could overflow int64 and panic on an adversarial
//     response; parseDynamicString rejects such payloads with a clean error.
//   - Format 2: fixed string (less common), the entire response is the
//     hex-encoded string. Attempted only when the dynamic decode fails.
func decodeTokenName(result string) (string, error) {
	// Remove 0x prefix
	result = strings.TrimPrefix(result, "0x")

	// If the result is empty or all zeros, return an error
	if len(result) == 0 || strings.TrimLeft(result, "0") == "" {
		return "", fmt.Errorf("empty result")
	}

	// Format 1: Dynamic string (most common)
	if name, decErr := parseDynamicString(result); decErr == nil {
		return name, nil
	}

	// Format 2: Fixed string (less common)
	// The entire response is the hex-encoded string
	if nameBytes, err := hex.DecodeString(strings.TrimRight(result, "0")); err == nil {
		return string(nameBytes), nil
	}

	return "", fmt.Errorf("failed to decode token name")
}

// GetTokenSymbol retrieves the symbol of an ERC20 token
func GetTokenSymbol(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SYMBOL)
	if err != nil {
		return "", err
	}
	return decodeTokenSymbol(result)
}

// decodeTokenSymbol decodes a symbol() qrl_call result. Split from
// GetTokenSymbol so the decode path is testable without a live node.
//
// Decodes via the hardened parseDynamicString. The previous hand-rolled
// decoder assumed a fixed 0x20 offset and sliced the data segment with no
// upper-bound check, so a malicious/broken contract returning a huge length
// word panicked the syncer; parseDynamicString bounds-checks every read and
// returns an error instead.
func decodeTokenSymbol(result string) (string, error) {
	// Decode the ABI-encoded string
	if len(result) < 130 {
		return "", fmt.Errorf("response too short")
	}
	return parseDynamicString(result)
}

// GetTokenDecimals retrieves the number of decimals for an ERC20 token
func GetTokenDecimals(contractAddress string) (uint8, error) {
	result, err := CallContractMethod(contractAddress, SIG_DECIMALS)
	if err != nil {
		return 0, err
	}
	return decodeTokenDecimals(result)
}

func decodeTokenDecimals(result string) (uint8, error) {
	decimals, err := decodeTokenUint256(result)
	if err != nil {
		return 0, fmt.Errorf("failed to decode token decimals: %w", err)
	}
	if decimals.BitLen() > 8 {
		return 0, fmt.Errorf("token decimals exceeds uint8: %s", decimals.String())
	}
	return uint8(decimals.Uint64()), nil
}

// GetTokenTotalSupply retrieves the total supply of an ERC20 token
func GetTokenTotalSupply(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SUPPLY)
	if err != nil {
		return "", err
	}
	return decodeTokenTotalSupply(result)
}

func decodeTokenTotalSupply(result string) (string, error) {
	totalSupply, err := decodeTokenUint256(result)
	if err != nil {
		return "", fmt.Errorf("failed to decode token total supply: %w", err)
	}
	return totalSupply.String(), nil
}

func decodeTokenBalance(result string) (string, error) {
	balance, err := decodeTokenUint256(result)
	if err != nil {
		return "", fmt.Errorf("failed to decode token balance: %w", err)
	}
	return balance.String(), nil
}

// decodeTokenUint256 accepts exactly one QIP-55 64-byte ABI word. Token
// scalars remain uint256 values in the low 32 bytes, with zero high bytes.
func decodeTokenUint256(result string) (*big.Int, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) != abiWordHexLength {
		return nil, fmt.Errorf(
			"ABI scalar result has %d hex chars, want %d",
			len(stripped),
			abiWordHexLength,
		)
	}
	return parseUint256FromWord(result)
}

// GetTokenBalance retrieves the balance of an ERC20 token for a specific address
func GetTokenBalance(contractAddress string, holderAddress string) (string, error) {
	return getTokenBalanceWithCaller(contractAddress, holderAddress, CallContractMethod)
}

// GetTokenBalanceAtBlock retrieves an ERC-20 balance from the exact state
// selected by blockHash.
func GetTokenBalanceAtBlock(
	contractAddress string,
	holderAddress string,
	blockHash string,
) (string, error) {
	return getTokenBalanceWithCaller(contractAddress, holderAddress,
		func(address string, calldata string) (string, error) {
			return callContractMethodAtBlockHash(address, calldata, blockHash)
		})
}

func getTokenBalanceWithCaller(
	contractAddress string,
	holderAddress string,
	call contractMethodCaller,
) (string, error) {
	// balanceOf(address) function signature
	methodID := "0x70a08231"

	// Enhanced logging with full input addresses
	zap.L().Debug("Getting token balance - raw input",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress))

	// Special handling for zero address (common in mint events)
	if validation.IsZeroAddress(holderAddress) {
		zap.L().Info("Zero address detected, returning zero balance",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress))
		return "0", nil
	}

	// Ensure contract address has Q prefix for QRL RPC
	contractAddress = validation.ConvertToQAddress(contractAddress)

	// Ensure holder address has Q prefix for RPC
	originalHolderAddress := holderAddress // Keep original for logging
	holderAddress = validation.ConvertToQAddress(holderAddress)

	// Encode the native address as one complete 64-byte ABI word.
	rawAddress := validation.StripAddressPrefix(holderAddress)
	encodedAddress := encodeAddressForABI(holderAddress)
	if encodedAddress == "" {
		return "", fmt.Errorf("invalid QIP-55 holder address: %s", originalHolderAddress)
	}

	// Combine method ID and the full-width address word.
	data := methodID + encodedAddress
	zap.L().Debug("Prepared contract call data",
		zap.String("contractAddress", contractAddress),
		zap.String("formattedAddress", holderAddress),
		zap.String("rawAddress", rawAddress),
		zap.String("encodedAddress", encodedAddress),
		zap.String("data", data))

	// Make the call. DoNodeRPC owns the retry + failover budget; a second
	// retry loop here only multiplied the delay on genuinely dead endpoints.
	result, err := call(contractAddress, data)
	if err != nil {
		zap.L().Error("Contract call for token balance failed after retries",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", originalHolderAddress),
			zap.String("formattedAddress", holderAddress),
			zap.String("encodedAddress", encodedAddress),
			zap.Error(err))
		return "", fmt.Errorf("contract call failed: %w", err)
	}

	balance, err := decodeTokenBalance(result)
	if err != nil {
		return "", err
	}
	zap.L().Info("Retrieved token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", originalHolderAddress),
		zap.String("balance", balance))

	return balance, nil
}
