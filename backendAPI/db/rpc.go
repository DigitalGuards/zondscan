package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"backendAPI/models"
)

// RPCError is a JSON-RPC error payload as returned by the QRL execution
// node. Surfaced as a distinct return value (rather than wrapped in a
// Go error) so callers can distinguish between transport failures and
// node-level errors (e.g. "execution reverted") that may carry useful
// metadata for the client.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// Default upstream HTTP client tuned for short-lived JSON-RPC calls.
// Pulled to package-level so every call shares the connection pool
// (keep-alive saves a TCP+TLS handshake per request).
var nodeRPCClient = &http.Client{Timeout: 12 * time.Second}

// NodeRPC issues a single JSON-RPC request to the configured node and
// returns the raw `result` field. Callers unmarshal `result` into whatever
// shape the method emits (string, hex, object, …).
//
// `ctx` controls cancellation/timeout in addition to the client's own
// 12s default — callers with per-call budgets shorter than that should
// pass a derived context.
//
// Returns:
//
//	(result, nil, nil)         on success
//	(nil, rpcErr, nil)         when the node returned a JSON-RPC error
//	(nil, nil, transportErr)   on network / marshalling failure
//
// `result` is `nil` only on error.
func NodeRPC(ctx context.Context, method string, params []interface{}) (json.RawMessage, *RPCError, error) {
	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://127.0.0.1:8545"
	}

	body, err := json.Marshal(models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal rpc body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nodeURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := nodeRPCClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("rpc transport: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("rpc transport: HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("rpc read body: %w", err)
	}

	var parsed struct {
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("rpc decode envelope: %w", err)
	}
	if parsed.Error != nil {
		return nil, parsed.Error, nil
	}
	if len(parsed.Result) == 0 {
		return nil, nil, errors.New("rpc envelope: missing result")
	}
	return parsed.Result, nil, nil
}
