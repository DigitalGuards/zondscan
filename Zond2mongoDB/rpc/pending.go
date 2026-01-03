package rpc

import (
	"Zond2mongoDB/models"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// GetPendingTransactions retrieves all pending transactions from the node
func GetPendingTransactions() string {
	// Get node URL from environment variable or use default
	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		zap.L().Error("NODE_URL environment variable not set")
		return ""
	}

	rpcReq := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_pendingTransactions",
		Params:  []interface{}{},
		ID:      1,
	}

	b, err := json.Marshal(rpcReq)
	if err != nil {
		zap.L().Error("Failed to marshal pending transactions request", zap.Error(err))
		return ""
	}

	req, err := http.NewRequest("POST", nodeURL, bytes.NewBuffer(b))
	if err != nil {
		zap.L().Error("Failed to create pending transactions request", zap.Error(err))
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		zap.L().Error("Failed to get pending transactions response", zap.Error(err))
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zap.L().Error("Unexpected status code from node", zap.Int("status_code", resp.StatusCode))
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("Failed to read pending transactions response", zap.Error(err))
		return ""
	}

	// Validate that we got a valid JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Error("Invalid JSON response from node", zap.Error(err))
		return ""
	}
	return string(body)
}
