package rpc

import (
	"QRL2MongoDB/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	traceRPCTimeout = 30 * time.Second
	traceRPCRetries = 3
)

// traceEndpoint returns the dedicated trace endpoint when configured. Public
// execution RPC endpoints commonly omit the debug namespace, so production
// deployments can expose debug only on a loopback WebSocket listener. The
// primary block-sync endpoint remains the backwards-compatible fallback.
func traceEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("TRACE_NODE_URL")); endpoint != "" {
		return endpoint
	}
	return Endpoints().PrimaryURL()
}

// traceRPC sends one JSON-RPC request to the trace endpoint. HTTP(S) keeps
// compatibility with nodes that already expose debug privately; WS(S) lets a
// node publish debug on a separate loopback listener while its public HTTP
// endpoint retains a restricted module list.
func traceRPC(ctx context.Context, request []byte) ([]byte, error) {
	endpoint := traceEndpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("no trace endpoint configured: set TRACE_NODE_URL or NODE_URL")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid trace endpoint: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < traceRPCRetries; attempt++ {
		switch parsed.Scheme {
		case "http", "https":
			// Retry here instead of in postWithRetry so TRACE_NODE_URL is never
			// included in retry logs. It may contain authentication material.
			returnBody, requestErr := postWithRetry(ctx, endpoint, request, 1)
			if requestErr == nil {
				return returnBody, nil
			}
			lastErr = redactTraceTransportError(endpoint, requestErr)
		case "ws", "wss":
			returnBody, requestErr := traceRPCWebSocket(ctx, endpoint, request)
			if requestErr == nil {
				return returnBody, nil
			}
			lastErr = redactTraceTransportError(endpoint, requestErr)
		default:
			return nil, fmt.Errorf("unsupported trace endpoint scheme %q", parsed.Scheme)
		}

		if attempt < traceRPCRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("trace endpoint failed after %d attempts: %w", traceRPCRetries, lastErr)
}

// redactTraceTransportError removes the configured endpoint, credentials, and
// query string from transport errors before they reach logs. Go HTTP and
// WebSocket errors often include the request URL.
func redactTraceTransportError(endpoint string, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	parsed, parseErr := url.Parse(endpoint)
	if parseErr != nil {
		return errors.New("trace transport failed")
	}

	message = strings.ReplaceAll(message, endpoint, "[trace endpoint]")
	if parsed.User != nil {
		message = strings.ReplaceAll(message, parsed.User.String(), "[credentials]")
	}
	if parsed.RawQuery != "" {
		message = strings.ReplaceAll(message, parsed.RawQuery, "[query]")
	}
	if parsed.Fragment != "" {
		message = strings.ReplaceAll(message, parsed.Fragment, "[fragment]")
	}
	return errors.New(message)
}

func traceRPCWebSocket(ctx context.Context, endpoint string, request []byte) ([]byte, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return nil, err
		}
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
	}

	if err := conn.WriteMessage(websocket.TextMessage, request); err != nil {
		return nil, err
	}
	_, response, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return response, nil
}

// ValidateDebugTraceEndpoint verifies the trace transport and module exposure
// before block synchronization starts. This turns a missing debug module into
// an immediate startup failure instead of silently dropping internal transfers
// from every subsequently indexed transaction.
func ValidateDebugTraceEndpoint(ctx context.Context) error {
	if os.Getenv("ENABLE_DEBUG_TRACE") != "true" {
		return nil
	}

	request, err := json.Marshal(models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "rpc_modules",
		Params:  []interface{}{},
		ID:      1,
	})
	if err != nil {
		return fmt.Errorf("marshal trace capability probe: %w", err)
	}

	responseBody, err := traceRPC(ctx, request)
	if err != nil {
		return fmt.Errorf("reach trace endpoint: %w", err)
	}

	var response struct {
		Result map[string]string `json:"result"`
		Error  *RPCError         `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("decode trace capability probe: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("trace capability probe failed: %w", response.Error)
	}
	if _, ok := response.Result["debug"]; !ok {
		return fmt.Errorf("trace endpoint does not expose the debug module; configure TRACE_NODE_URL with a private debug-enabled RPC endpoint")
	}
	return nil
}
