package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
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

	if len(result) < 66 {
		return 0, fmt.Errorf("response too short")
	}

	decimals, err := strconv.ParseUint(result[2:], 16, 8)
	if err != nil {
		return 0, err
	}

	return uint8(decimals), nil
}

// GetTokenTotalSupply retrieves the total supply of an ERC20 token
func GetTokenTotalSupply(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SUPPLY)
	if err != nil {
		return "", err
	}

	if len(result) < 66 {
		return "", fmt.Errorf("response too short")
	}

	// Convert hex to decimal
	bigInt := new(big.Int)
	if _, ok := bigInt.SetString(strings.TrimPrefix(result, "0x"), 16); !ok {
		return "", fmt.Errorf("failed to parse total supply")
	}

	// Return decimal string
	return bigInt.String(), nil
}

// GetTokenBalance retrieves the balance of an ERC20 token for a specific address
func GetTokenBalance(contractAddress string, holderAddress string) (string, error) {
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

	// Extract the raw address (without prefix) for padding
	rawAddress := validation.StripAddressPrefix(holderAddress)

	// Pad address to 32 bytes (64 hex chars) for ABI encoding
	paddedAddress := rawAddress
	for len(paddedAddress) < 64 {
		paddedAddress = "0" + paddedAddress
	}

	// Combine method ID and padded address
	data := methodID + paddedAddress
	zap.L().Debug("Prepared contract call data",
		zap.String("contractAddress", contractAddress),
		zap.String("formattedAddress", holderAddress),
		zap.String("rawAddress", rawAddress),
		zap.String("paddedAddress", paddedAddress),
		zap.String("data", data))

	// Make the call. DoNodeRPC owns the retry + failover budget; a second
	// retry loop here only multiplied the delay on genuinely dead endpoints.
	result, err := CallContractMethod(contractAddress, data)
	if err != nil {
		zap.L().Error("Contract call for token balance failed after retries",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", originalHolderAddress),
			zap.String("formattedAddress", holderAddress),
			zap.String("paddedAddress", paddedAddress),
			zap.Error(err))
		return "", fmt.Errorf("contract call failed: %v", err)
	}

	// Parse result
	if len(result) < 2 {
		zap.L().Warn("Empty result from token balance call",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", originalHolderAddress))
		return "0", nil
	}

	// Convert hex string to big.Int
	bigInt := new(big.Int)
	bigInt.SetString(strings.TrimPrefix(result, "0x"), 16)

	balance := bigInt.String()
	zap.L().Info("Retrieved token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", originalHolderAddress),
		zap.String("balance", balance))

	return balance, nil
}
