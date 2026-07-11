package rpc

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/validation"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Package-level HTTP client with connection pooling and timeouts
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	},
}

// GetHTTPClient returns the package-level HTTP client
func GetHTTPClient() *http.Client {
	return httpClient
}

type MyHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// CallContractMethod makes a qrl_call to a contract method and returns the result
func CallContractMethod(contractAddress string, methodSig string) (string, error) {
	zap.L().Debug("Calling contract method",
		zap.String("contractAddress", contractAddress),
		zap.String("methodSig", methodSig[:10]+"...")) // Log just the beginning of the signature for brevity

	// Ensure contract address has Q prefix for QRL RPC
	contractAddress = validation.ConvertToQAddress(contractAddress)

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "qrl_call",
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

	zap.L().Debug("Sending RPC request",
		zap.String("url", Endpoints().CurrentURL()),
		zap.String("method", "qrl_call"))

	body, err := DoNodeRPC(b)
	if err != nil {
		zap.L().Error("Failed to execute HTTP request for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to execute request: %v", err)
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

// GetTransactionReceipt gets the transaction receipt which includes logs
func GetTransactionReceipt(txHash string) (*models.TransactionReceipt, error) {
	if txHash == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "qrl_getTransactionReceipt",
		Params:  []interface{}{txHash},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	body, err := DoNodeRPC(b)
	if err != nil {
		return nil, fmt.Errorf("failed to make RPC request: %v", err)
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
