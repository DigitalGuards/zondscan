package rpc

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/networkprofile"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// GetPendingTransactions retrieves all pending transactions from the node.
// Uses MEMPOOL_NODE_URL if set, otherwise falls back to the configured primary
// node URL. txpool_content is a primary-only RPC method (foundation public RPC
// does not expose txpool_*), so this never fails over.
func GetPendingTransactions() string {
	// Try MEMPOOL_NODE_URL first (for dedicated mempool node), fall back to the
	// primary URL from the failover-aware selector (covers both NODE_URLS list
	// and legacy NODE_URL env vars).
	nodeURL := os.Getenv("MEMPOOL_NODE_URL")
	if nodeURL == "" {
		nodeURL = Endpoints().PrimaryURL()
	}
	if nodeURL == "" {
		zap.L().Error("No mempool URL: set MEMPOOL_NODE_URL, NODE_URL, or NODE_URLS")
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := networkprofile.GuardSource(ctx, nodeURL); err != nil {
		zap.L().Error("Mempool network identity mismatch", zap.Error(err))
		return ""
	}

	// Use txpool_content which actually works on Zond nodes
	// Pending transactions are fetched via txpool_content (not a dedicated pending RPC method)
	rpcReq := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "txpool_content",
		Params:  []interface{}{},
		ID:      1,
	}

	b, err := json.Marshal(rpcReq)
	if err != nil {
		zap.L().Error("Failed to marshal pending transactions request", zap.Error(err))
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, "POST", nodeURL, bytes.NewBuffer(b))
	if err != nil {
		zap.L().Error("Failed to create pending transactions request", zap.Error(err))
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
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

	if err := networkprofile.GuardSource(ctx, nodeURL); err != nil {
		zap.L().Error("Mempool network identity mismatch", zap.Error(err))
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
