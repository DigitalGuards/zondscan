package rpc

import (
	"Zond2mongoDB/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"
)

const (
	// ERC20 factory events
	TokenDeployedEventSignature1 = "0xf1a761556f754e3f6ffaff60863242d8680a8e7b2362e5100c1744144498c60f" // TokenDeployed(address,address)
	TokenDeployedEventSignature2 = "0xc896d0ddb9c4eb2c74a57a74a4008c30ba7675d3d5d20226b39a1695a35fc10e" // TokenCreated(address,address,string,string)

	// Direct token creation method signature
	TokenCreationSignature = "0x48d81308" // Token creation function signature

	// Token transfer method signature - also in DecodeTransferEvent
	TokenTransferSignature = "0xa9059cbb" // transfer(address,uint256)

	// Common factory creation events - not hardcoded to specific factory addresses
	// PairCreated event signature: keccak256("PairCreated(address,address,address,uint256)")
	PairCreatedEventSignature = "0x0d3648bd0f6ba80134a33ba9275ac585d9d315f0ad8355cddefde31afa28d0e9"
)

// ZondGetLogs retrieves logs with specific filters
func ZondGetLogs(fromBlock, toBlock string, addresses []string, topics []string) (*models.LogsResponse, error) {
	zap.L().Debug("Querying logs with filters",
		zap.String("fromBlock", fromBlock),
		zap.String("toBlock", toBlock),
		zap.Strings("addresses", addresses),
		zap.Strings("topics", topics))

	// Build filter object
	filterObj := map[string]interface{}{
		"fromBlock": fromBlock,
		"toBlock":   toBlock,
	}

	// Add addresses if provided
	if len(addresses) > 0 {
		filterObj["address"] = addresses
	}

	// Add topics if provided
	if len(topics) > 0 {
		filterObj["topics"] = []interface{}{topics[0]} // Use first topic as the primary filter
	}

	// Create JSON-RPC request
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getLogs",
		Params:  []interface{}{filterObj},
		ID:      1,
	}

	// Marshal request to JSON
	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Error("Failed to marshal JSON for getLogs",
			zap.Error(err))
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Get RPC endpoint
	nodeUrl := os.Getenv("NODE_URL")
	zap.L().Debug("Sending getLogs RPC request",
		zap.String("url", nodeUrl))

	// Create HTTP request
	req, err := http.NewRequest("POST", nodeUrl, bytes.NewBuffer(b))
	if err != nil {
		zap.L().Error("Failed to create HTTP request for getLogs",
			zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Error("Failed to execute HTTP request for getLogs",
			zap.Error(err))
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("Failed to read response body from getLogs",
			zap.Error(err))
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Unmarshal response
	var logsResponse models.LogsResponse
	if err := json.Unmarshal(body, &logsResponse); err != nil {
		zap.L().Error("Failed to unmarshal response from getLogs",
			zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	// Check for RPC errors
	if logsResponse.Error != nil {
		zap.L().Error("RPC error in getLogs",
			zap.Int("errorCode", logsResponse.Error.Code),
			zap.String("errorMessage", logsResponse.Error.Message))
		return nil, fmt.Errorf("RPC error: %v", logsResponse.Error.Message)
	}

	zap.L().Info("Retrieved logs successfully",
		zap.Int("logCount", len(logsResponse.Result)))
	return &logsResponse, nil
}
