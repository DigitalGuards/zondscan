package db

import (
	"backendAPI/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func GetPendingTransactions() (*models.PendingTransactionsResponse, error) {
	// Create JSON-RPC request
	rpcReq := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "txpool_content",
		Params:  []interface{}{},
		ID:      1,
	}

	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Get node URL from environment variable or use default
	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://REDACTED:8545" // fallback to default if not set
	}

	// Create and send HTTP request
	req, err := http.NewRequest("POST", nodeURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Parse response
	var result models.PendingTransactionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	return &result, nil
}
