package rpc

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/validation"
	"encoding/json"
	"errors"
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

// RPCError is a JSON-RPC error member surfaced as a typed error, so callers
// can distinguish node-reported errors (e.g. contract reverts on qrl_call)
// from transport failures with errors.As instead of sniffing formatted
// strings. The Error() text keeps the historical "RPC error: <message>"
// shape logs and operators already know.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error: %v", e.Message)
}

// isNodeRPCError reports whether err is a node-reported JSON-RPC error (for
// qrl_call that means the contract reverted) as opposed to a transport
// failure. Callers use this to implement the three-way error contract
// documented on GetERC721Owner / SupportsInterface.
func isNodeRPCError(err error) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr)
}

// rpcCall marshals a JSON-RPC request, posts it via DoNodeRPC (retry +
// failover), surfaces a JSON-RPC error member as *RPCError, and unmarshals
// the full response body into out (pass nil to skip decoding). Every typed
// node call should go through this instead of hand-rolling the plumbing.
func rpcCall(method string, params []interface{}, out interface{}) error {
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	body, err := DoNodeRPC(b)
	if err != nil {
		return err
	}

	// Decode the error member separately: most call-site response structs
	// only model the result shape.
	var envelope struct {
		Error *RPCError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}
	if envelope.Error != nil {
		return envelope.Error
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed to unmarshal response: %v", err)
		}
	}
	return nil
}

// CallContractMethod makes a qrl_call to a contract method and returns the result
func CallContractMethod(contractAddress string, methodSig string) (string, error) {
	zap.L().Debug("Calling contract method",
		zap.String("contractAddress", contractAddress),
		zap.String("methodSig", methodSig[:10]+"...")) // Log just the beginning of the signature for brevity

	// Ensure contract address has Q prefix for QRL RPC
	contractAddress = validation.ConvertToQAddress(contractAddress)

	var result struct {
		Result string
	}
	err := rpcCall("qrl_call", []interface{}{
		map[string]string{
			"to":   contractAddress,
			"data": methodSig,
		},
		"latest",
	}, &result)
	if err != nil {
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) {
			zap.L().Error("RPC error in contract call",
				zap.String("contractAddress", contractAddress),
				zap.Int("errorCode", rpcErr.Code),
				zap.String("errorMessage", rpcErr.Message))
		} else {
			zap.L().Error("Failed to execute contract call",
				zap.String("contractAddress", contractAddress),
				zap.Error(err))
		}
		return "", err
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

	var receipt models.TransactionReceipt
	if err := rpcCall("qrl_getTransactionReceipt", []interface{}{txHash}, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}
