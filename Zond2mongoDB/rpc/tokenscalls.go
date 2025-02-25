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
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

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
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("RPC error: %v", result.Error.Message)
	}

	return result.Result, nil
}

// GetTokenInfo attempts to get ERC20 token information for a contract
func GetTokenInfo(contractAddress string) (name string, symbol string, decimals uint8, isToken bool) {
	var err error
	name, err = GetTokenName(contractAddress)
	if err != nil {
		zap.L().Debug("Failed to get token name", zap.Error(err))
		return "", "", 0, false
	}

	symbol, err = GetTokenSymbol(contractAddress)
	if err != nil {
		zap.L().Debug("Failed to get token symbol", zap.Error(err))
		return "", "", 0, false
	}

	decimals, err = GetTokenDecimals(contractAddress)
	if err != nil {
		zap.L().Debug("Failed to get token decimals", zap.Error(err))
		return "", "", 0, false
	}

	// If we got here, this is likely a valid token
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

	// Remove 0x prefix and pad address to 32 bytes
	address := strings.TrimPrefix(holderAddress, "0x")
	for len(address) < 64 {
		address = "0" + address
	}

	// Combine method ID and padded address
	data := methodID + address

	// Make the call
	result, err := CallContractMethod(contractAddress, data)
	if err != nil {
		return "", fmt.Errorf("contract call failed: %v", err)
	}

	// Parse result
	if len(result) < 2 {
		return "0", nil
	}

	// Convert hex string to big.Int
	bigInt := new(big.Int)
	bigInt.SetString(strings.TrimPrefix(result, "0x"), 16)

	return bigInt.String(), nil
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
		recipient := "0x" + trimLeftZeros(data[34:74])
		if len(recipient) != 42 { // Check if it's a valid address length (0x + 40 hex chars)
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
			// Topics[1] is from address (padded to 32 bytes)
			from := "0x" + trimLeftZeros(log.Topics[1][26:])

			// Topics[2] is to address (padded to 32 bytes)
			to := "0x" + trimLeftZeros(log.Topics[2][26:])

			// Log.Data contains the amount (32 bytes)
			amount := log.Data

			if len(from) == 42 && len(to) == 42 {
				transfers = append(transfers, TransferEvent{
					From:   from,
					To:     to,
					Amount: amount,
				})
			}
		}
	}

	return transfers
}

type TransferEvent struct {
	From   string
	To     string
	Amount string
}

// Helper function to trim leading zeros from hex string
func trimLeftZeros(hex string) string {
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

	// Ensure addresses have proper format
	if !strings.HasPrefix(from, "0x") && !strings.HasPrefix(from, "Z") {
		from = "0x" + from
	}

	if !strings.HasPrefix(to, "0x") && !strings.HasPrefix(to, "Z") {
		to = "0x" + to
	}

	// Validate addresses
	if !validation.IsValidAddress(from) {
		return "", "", nil, fmt.Errorf("invalid from address: %s", from)
	}

	if !validation.IsValidAddress(to) {
		return "", "", nil, fmt.Errorf("invalid to address: %s", to)
	}

	// ... existing code ...
	return "", "", nil, nil
}
