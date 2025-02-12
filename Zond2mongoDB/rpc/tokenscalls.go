package rpc

import (
	"Zond2mongoDB/models"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Common ERC20 function signatures
const (
	SIG_NAME     = "0x06fdde03" // name()
	SIG_SYMBOL   = "0x95d89b41" // symbol()
	SIG_DECIMALS = "0x313ce567" // decimals()
	SIG_BALANCE  = "0x70a08231" // balanceOf(address)
	SIG_SUPPLY   = "0x18160ddd" // totalSupply()
)

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

	resp, err := http.DefaultClient.Do(req)
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
	nameBytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", err
	}

	return string(nameBytes), nil
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

	// Total supply is a uint256, return the hex string as is
	return result, nil
}

// GetTokenBalance retrieves the balance of an ERC20 token for a specific address
func GetTokenBalance(contractAddress string, holderAddress string) (string, error) {
	// Pad the address to 32 bytes
	paddedAddress := fmt.Sprintf("%s%s", SIG_BALANCE, strings.TrimPrefix(holderAddress, "0x"))
	for len(paddedAddress) < 74 { // 2 (0x) + 8 (function selector) + 64 (32 bytes)
		paddedAddress = paddedAddress + "0"
	}

	result, err := CallContractMethod(contractAddress, paddedAddress)
	if err != nil {
		return "", err
	}

	if len(result) < 66 {
		return "", fmt.Errorf("response too short")
	}

	// Balance is a uint256, return the hex string as is
	return result, nil
}
