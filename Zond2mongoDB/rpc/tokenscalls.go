package rpc

import (
	"Zond2mongoDB/models"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"Zond2mongoDB/validation"

	"go.uber.org/zap"
)

// Method signatures for ERC20 token functions
const (
	SIG_NAME     = "0x06fdde03" // name()
	SIG_SYMBOL   = "0x95d89b41" // symbol()
	SIG_DECIMALS = "0x313ce567" // decimals()
	SIG_BALANCE  = "0x70a08231" // balanceOf(address)
	SIG_SUPPLY   = "0x18160ddd" // totalSupply()
)

// Transfer event signature: keccak256("Transfer(address,address,uint256)")
const TransferEventSignature = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

// CallContractMethod makes a zond_call to a contract method and returns the result
func CallContractMethod(contractAddress string, methodSig string) (string, error) {
	zap.L().Debug("Calling contract method",
		zap.String("contractAddress", contractAddress),
		zap.String("methodSig", methodSig[:10]+"...")) // Log just the beginning of the signature for brevity

	// Ensure contract address has correct format for Zond blockchain
	// For Zond RPC calls, we need the correct prefix
	if !strings.HasPrefix(contractAddress, "Z") && strings.HasPrefix(contractAddress, "0x") {
		contractAddress = "Z" + strings.TrimPrefix(contractAddress, "0x")
	} else if !strings.HasPrefix(contractAddress, "Z") && !strings.HasPrefix(contractAddress, "0x") {
		contractAddress = "Z" + contractAddress
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_call",
		Params: []interface{}{
			map[string]string{
				"to":   contractAddress,
				"data": methodSig,
			},
			"latest",
		},
		ID: 1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Error("Failed to marshal JSON for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Log the RPC endpoint
	nodeUrl := os.Getenv("NODE_URL")
	zap.L().Debug("Sending RPC request",
		zap.String("url", nodeUrl),
		zap.String("method", "zond_call"))

	req, err := http.NewRequest("POST", nodeUrl, bytes.NewBuffer(b))
	if err != nil {
		zap.L().Error("Failed to create HTTP request for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Error("Failed to execute HTTP request for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("Failed to read response body from contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log full response for debugging
	zap.L().Debug("Received contract call response",
		zap.String("contractAddress", contractAddress),
		zap.String("response", string(body)))

	var result struct {
		Jsonrpc string
		ID      int
		Result  string
		Error   *struct {
			Code    int
			Message string
		}
	}
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Error("Failed to unmarshal response from contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if result.Error != nil {
		zap.L().Error("RPC error in contract call",
			zap.String("contractAddress", contractAddress),
			zap.Int("errorCode", result.Error.Code),
			zap.String("errorMessage", result.Error.Message))
		return "", fmt.Errorf("RPC error: %v", result.Error.Message)
	}

	// Truncate the result for logging if it's too long
	resultForLog := result.Result
	if len(resultForLog) > 100 {
		resultForLog = resultForLog[:100] + "..."
	}
	zap.L().Debug("Contract call successful",
		zap.String("contractAddress", contractAddress),
		zap.String("result", resultForLog))

	return result.Result, nil
}

// GetTokenInfo attempts to determine if a contract is an ERC20 token and returns its details
func GetTokenInfo(contractAddress string) (string, string, uint8, bool) {
	zap.L().Info("Checking if contract is a token", zap.String("address", contractAddress))

	// First check if the contract has a valid 'name' method
	name, err := GetTokenName(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid name method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has a valid name",
		zap.String("address", contractAddress),
		zap.String("name", name))

	// Now check for symbol
	symbol, err := GetTokenSymbol(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid symbol method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has a valid symbol",
		zap.String("address", contractAddress),
		zap.String("symbol", symbol))

	// Finally check for decimals
	decimals, err := GetTokenDecimals(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid decimals method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has valid decimals",
		zap.String("address", contractAddress),
		zap.Uint8("decimals", decimals))

	// If we got here, this is likely a valid token
	zap.L().Info("Detected valid ERC20 token",
		zap.String("address", contractAddress),
		zap.String("name", name),
		zap.String("symbol", symbol),
		zap.Uint8("decimals", decimals))

	return name, symbol, decimals, true
}

// GetTokenName retrieves the name of an ERC20 token
func GetTokenName(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_NAME)
	if err != nil {
		return "", err
	}

	// Remove 0x prefix
	result = strings.TrimPrefix(result, "0x")

	// If the result is empty or all zeros, return an error
	if len(result) == 0 || strings.TrimLeft(result, "0") == "" {
		return "", fmt.Errorf("empty result")
	}

	// Handle different response formats:

	// Format 1: Dynamic string (most common)
	// First 32 bytes (64 chars) contain the offset to the string data
	// Next 32 bytes contain the string length
	// Followed by the string data
	if len(result) >= 128 {
		// Try parsing as dynamic string
		offsetHex := result[:64]
		offset, err := strconv.ParseInt(offsetHex, 16, 64)
		if err == nil && offset*2 < int64(len(result)) {
			// Get the length from the offset position
			startPos := offset * 2
			if startPos+64 <= int64(len(result)) {
				lengthHex := result[startPos : startPos+64]
				length, err := strconv.ParseInt(lengthHex, 16, 64)
				if err == nil && startPos+64+length*2 <= int64(len(result)) {
					dataHex := result[startPos+64 : startPos+64+length*2]
					if nameBytes, err := hex.DecodeString(dataHex); err == nil {
						return string(nameBytes), nil
					}
				}
			}
		}
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

	// Decode the ABI-encoded string
	if len(result) < 130 {
		return "", fmt.Errorf("response too short")
	}

	// Extract the string length and data
	dataStart := 2 + 64 // skip "0x" and first 32 bytes
	lengthHex := result[dataStart : dataStart+64]
	length, err := strconv.ParseInt(lengthHex, 16, 64)
	if err != nil {
		return "", err
	}

	dataHex := result[dataStart+64 : dataStart+64+int(length)*2]
	symbolBytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", err
	}

	return string(symbolBytes), nil
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
	// Handle multiple formats of zero address
	if holderAddress == "0x0" ||
		holderAddress == "0x0000000000000000000000000000000000000000" ||
		holderAddress == "Z0" ||
		holderAddress == "Z0000000000000000000000000000000000000000" ||
		strings.ToLower(holderAddress) == "0x0000000000000000000000000000000000000000" {
		zap.L().Info("Zero address detected, returning zero balance",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress))
		return "0", nil
	}

	// For Zond blockchain, ensure contract address uses Z prefix, not 0x
	if strings.HasPrefix(contractAddress, "0x") {
		contractAddress = "Z" + strings.TrimPrefix(contractAddress, "0x")
	} else if !strings.HasPrefix(contractAddress, "Z") {
		// Add Z prefix if missing
		contractAddress = "Z" + contractAddress
	}

	// Ensure holder address uses Z prefix for Zond blockchain compatibility
	originalHolderAddress := holderAddress // Keep original for logging

	// 1. Extract the address without any prefix
	var rawAddress string
	if strings.HasPrefix(holderAddress, "0x") {
		rawAddress = strings.TrimPrefix(holderAddress, "0x")
	} else if strings.HasPrefix(holderAddress, "Z") {
		rawAddress = strings.TrimPrefix(holderAddress, "Z")
	} else {
		rawAddress = holderAddress
	}

	// 2. Ensure the raw address is the correct length
	if len(rawAddress) > 40 {
		rawAddress = rawAddress[:40]
	} else if len(rawAddress) < 40 {
		// Pad with leading zeros to reach 40 characters
		for len(rawAddress) < 40 {
			rawAddress = "0" + rawAddress
		}
	}

	// 3. Format for RPC call - for Zond, ensure it has Z prefix
	holderAddress = "Z" + rawAddress

	// Pad address to 32 bytes (64 hex chars) for ABI encoding
	// First remove any prefix
	paddedAddress := rawAddress
	// Then pad to 64 chars total
	for len(paddedAddress) < 64 {
		paddedAddress = "0" + paddedAddress
	}

	// Combine method ID and padded address
	data := methodID + paddedAddress
	zap.L().Debug("Prepared contract call data",
		zap.String("contractAddress", contractAddress),
		zap.String("formattedAddress", holderAddress),
		zap.String("data", data))

	// Make the call
	result, err := CallContractMethod(contractAddress, data)
	if err != nil {
		// Try up to 3 times with exponential backoff on failure
		maxRetries := 2
		for retry := 0; retry < maxRetries && err != nil; retry++ {
			retryDelay := time.Duration(500*(retry+1)) * time.Millisecond

			zap.L().Warn("Retrying token balance call after failure",
				zap.String("contractAddress", contractAddress),
				zap.String("holderAddress", holderAddress),
				zap.Int("retry", retry+1),
				zap.Duration("delay", retryDelay),
				zap.Error(err))

			time.Sleep(retryDelay)
			result, err = CallContractMethod(contractAddress, data)
		}

		// If all retries failed
		if err != nil {
			zap.L().Error("Contract call for token balance failed after retries",
				zap.String("contractAddress", contractAddress),
				zap.String("holderAddress", originalHolderAddress),
				zap.String("formattedAddress", holderAddress),
				zap.String("paddedAddress", paddedAddress),
				zap.Error(err))
			return "", fmt.Errorf("contract call failed: %v", err)
		}
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

// DecodeTransferEvent decodes token transfers from both:
// 1. Direct transfer calls (tx.data starting with 0xa9059cbb)
// 2. Transfer events in transaction logs
func DecodeTransferEvent(data string) (string, string, string) {
	// First try to decode direct transfer call
	if len(data) >= 10 && data[:10] == "0xa9059cbb" {
		if len(data) != 138 { // 2 (0x) + 8 (func) + 64 (to) + 64 (amount) = 138
			return "", "", ""
		}

		// Extract recipient address (remove leading zeros)
		recipient := "Z" + TrimLeftZeros(data[34:74])
		if len(recipient) != 41 { // Check if it's a valid address length (Z + 40 hex chars)
			return "", "", ""
		}

		// Extract amount
		amount := "0x" + data[74:]
		return "", recipient, amount
	}

	return "", "", ""
}

// GetTransactionReceipt gets the transaction receipt which includes logs
func GetTransactionReceipt(txHash string) (*models.TransactionReceipt, error) {
	if txHash == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}

	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		return nil, fmt.Errorf("NODE_URL environment variable is not set")
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getTransactionReceipt",
		Params:  []interface{}{txHash},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make HTTP request
	resp, err := GetHTTPClient().Post(nodeURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("failed to make RPC request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// First unmarshal into a map to check for JSON-RPC error
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Check for JSON-RPC error
	if errObj, ok := rawResponse["error"]; ok {
		return nil, fmt.Errorf("RPC error: %v", errObj)
	}

	var receipt models.TransactionReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %v", err)
	}

	return &receipt, nil
}

// ProcessTransferLogs processes Transfer events from transaction logs
func ProcessTransferLogs(receipt *models.TransactionReceipt) []TransferEvent {
	var transfers []TransferEvent

	for _, log := range receipt.Result.Logs {
		// Check if this is a Transfer event
		if len(log.Topics) == 3 && log.Topics[0] == TransferEventSignature {
			from, to, amount, err := ParseTransferEvent(log)
			if err != nil {
				// Log the error but continue processing other logs
				zap.L().Error("Failed to parse transfer event", zap.Error(err))
				continue
			}

			transfers = append(transfers, TransferEvent{
				From:   from,
				To:     to,
				Amount: amount.String(),
			})
		}
	}

	return transfers
}

type TransferEvent struct {
	From   string
	To     string
	Amount string
}

// TrimLeftZeros trims leading zeros from hex string
func TrimLeftZeros(hex string) string {
	for i := 0; i < len(hex); i++ {
		if hex[i] != '0' {
			return hex[i:]
		}
	}
	return "0"
}

// IsValidRecipient checks if a recipient address is valid
func IsValidRecipient(recipient string) bool {
	return validation.IsValidAddress(recipient)
}

// ParseTransferEvent parses a transfer event log
func ParseTransferEvent(log models.Log) (string, string, *big.Int, error) {
	// Extract addresses from topics
	from := log.Topics[1]
	to := log.Topics[2]

	// Ensure addresses have proper format with Z prefix (not 0x)
	if !strings.HasPrefix(from, "Z") {
		from = "Z" + TrimLeftZeros(from)
	}

	if !strings.HasPrefix(to, "Z") {
		to = "Z" + TrimLeftZeros(to)
	}

	// Validate addresses
	if !validation.IsValidAddress(from) {
		return "", "", nil, fmt.Errorf("invalid from address: %s", from)
	}

	if !validation.IsValidAddress(to) {
		return "", "", nil, fmt.Errorf("invalid to address: %s", to)
	}

	// Parse amount from data field
	amount := new(big.Int)
	if len(log.Data) > 2 {
		// Remove 0x prefix if present
		data := log.Data
		if strings.HasPrefix(data, "0x") {
			data = data[2:]
		}

		// Set the value from hex string
		if _, success := amount.SetString(data, 16); !success {
			return "", "", nil, fmt.Errorf("failed to parse amount from data: %s", log.Data)
		}
	}

	return from, to, amount, nil
}
