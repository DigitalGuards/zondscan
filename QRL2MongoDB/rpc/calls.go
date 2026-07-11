package rpc

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func GetLatestBlock() (string, error) {
	var Zond models.RPC
	if err := rpcCall("qrl_blockNumber", []interface{}{}, &Zond); err != nil {
		zap.L().Error("Failed to get response from RPC call after retries/failover", zap.Error(err))
		return "0x0", err
	}

	// Validate response format
	if !validation.IsValidHexString(Zond.Result) {
		return "0x0", fmt.Errorf("invalid block number format in response: %s", Zond.Result)
	}

	return Zond.Result, nil
}

func GetBlockByNumberMainnet(blockNumber string) (*models.ZondDatabaseBlock, error) {
	// Validate block number format
	if !validation.IsValidHexString(blockNumber) {
		return nil, fmt.Errorf("invalid block number format: %s", blockNumber)
	}

	var block models.ZondDatabaseBlock
	if err := rpcCall("qrl_getBlockByNumber", []interface{}{blockNumber, true}, &block); err != nil {
		zap.L().Info("Failed to get response from RPC call", zap.Error(err))
		return nil, err
	}

	// Validate block hash format
	if err := validation.ValidateHexString(block.Result.Hash, validation.HashLength); err != nil {
		return nil, fmt.Errorf("invalid block hash: %v", err)
	}

	// Validate transaction hashes
	for _, tx := range block.Result.Transactions {
		if err := validation.ValidateHexString(tx.Hash, validation.HashLength); err != nil {
			return nil, fmt.Errorf("invalid transaction hash: %v", err)
		}
		if tx.To != "" {
			if err := validation.ValidateAddress(tx.To); err != nil {
				return nil, fmt.Errorf("invalid to address: %v", err)
			}
		}
		if err := validation.ValidateAddress(tx.From); err != nil {
			return nil, fmt.Errorf("invalid from address: %v", err)
		}
	}

	return &block, nil
}

func GetContractAddress(txHash string) (string, string, error) {
	// Validate input transaction hash
	if err := validation.ValidateHexString(txHash, validation.HashLength); err != nil {
		return "", "", fmt.Errorf("invalid transaction hash: %v", err)
	}
	var ContractAddress models.Contract
	if err := rpcCall("qrl_getTransactionReceipt", []interface{}{txHash}, &ContractAddress); err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", "", err
	}

	// Validate contract address if present
	if ContractAddress.Result.ContractAddress != "" {
		if err := validation.ValidateAddress(ContractAddress.Result.ContractAddress); err != nil {
			return "", "", fmt.Errorf("invalid contract address in response: %v", err)
		}
	}

	// Validate status format
	if ContractAddress.Result.Status != "" && !validation.IsValidHexString(ContractAddress.Result.Status) {
		return "", "", fmt.Errorf("invalid status format in response: %s", ContractAddress.Result.Status)
	}

	return ContractAddress.Result.ContractAddress, ContractAddress.Result.Status, nil
}

// DebugTraceResult holds all data returned by CallDebugTraceTransaction.
// Using a struct avoids the fragility of 12 positional return values and
// makes call-sites readable without relying on positional assignment.
type DebugTraceResult struct {
	// TransactionType is the EVM call type string (e.g. "CALL", "CREATE").
	TransactionType string
	// CallType is the sub-call type (e.g. "call", "delegatecall").
	CallType string
	// From is the caller address in Q-prefix format.
	From string
	// To is the callee address in Q-prefix format.
	To string
	// Input is always 0 in the current implementation (field reserved for future use).
	Input uint64
	// Output is a uint64 representation of the call output (1 = success/non-empty, 0 = empty/failure).
	Output uint64
	// TraceAddress is the call-tree position of this call frame.
	TraceAddress []int
	// Value is the ETH/QRL value transferred, scaled by QUANTA.
	Value float32
	// Gas is the gas supplied to the call frame.
	Gas uint64
	// GasUsed is the gas consumed by the call frame.
	GasUsed uint64
	// AddressFunctionIdentifier is the recipient address decoded from the transfer() input data.
	AddressFunctionIdentifier string
	// AmountFunctionIdentifier is the transfer amount decoded from the transfer() input data.
	AmountFunctionIdentifier uint64
	// InternalCalls are the nested (depth >= 1) call frames worth persisting
	// as internal transactions: value transfers executed by contract code and
	// contract creations. The top-level frame is not included; it already
	// exists as the outer transaction row.
	InternalCalls []InternalCall
	// Err is non-nil when the RPC call itself failed (network error, unmarshal error, etc.).
	// A nil Err with empty TransactionType means the trace has no relevant call data.
	Err error
}

// InternalCall is one nested callTracer frame, flattened for persistence in
// the internalTransactionByAddress collection.
type InternalCall struct {
	// Type is the frame opcode (CALL, CALLCODE, CREATE, CREATE2, SELFDESTRUCT).
	Type string
	// From and To are Q-prefix addresses.
	From string
	To   string
	// Input and Output are raw hex strings ("0x" when absent).
	Input  string
	Output string
	// TraceAddress is the frame's position in the call tree, derived from
	// child indices (callTracer itself does not emit it).
	TraceAddress []int
	// Value is the QRL amount moved by this frame (wei / QUANTA), matching
	// the float schema of the existing collection.
	Value float64
	// Gas and GasUsed are raw hex strings ("0x0" when absent).
	Gas     string
	GasUsed string
}

// parseHexBig parses a 0x-prefixed hex quantity into a big.Int, or nil when
// the input is empty or malformed.
func parseHexBig(hexVal string) *big.Int {
	if hexVal == "" || !validation.IsValidHexString(hexVal) {
		return nil
	}
	v := new(big.Int)
	if _, ok := v.SetString(strings.TrimPrefix(hexVal, "0x"), 16); !ok {
		return nil
	}
	return v
}

// hexOrDefault returns the hex string unchanged, or def when empty.
func hexOrDefault(hexVal string, def string) string {
	if hexVal == "" {
		return def
	}
	return hexVal
}

// flattenCalls walks a callTracer call tree depth-first and returns the
// nested frames worth persisting as internal transactions: frames that move
// value plus contract creations. Frames with a non-empty error are skipped
// together with their whole subtree, a reverted frame's transfers never took
// effect. path is the traceAddress prefix inherited from the parent frame.
func flattenCalls(calls []models.Call, path []int) []InternalCall {
	var out []InternalCall
	for i, call := range calls {
		if call.Error != "" {
			continue
		}

		tracePath := make([]int, 0, len(path)+1)
		tracePath = append(tracePath, path...)
		tracePath = append(tracePath, i)

		value := parseHexBig(call.Value)
		hasValue := value != nil && value.Sign() > 0
		if hasValue || strings.HasPrefix(call.Type, "CREATE") {
			valueFloat := 0.0
			if hasValue {
				divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
				quo := new(big.Float).Quo(new(big.Float).SetInt(value), divisor)
				valueFloat, _ = quo.Float64()
			}

			from := call.From
			if from != "" {
				if err := validation.ValidateAddress(from); err != nil {
					zap.L().Warn("Invalid from address in internal call frame", zap.Error(err))
				}
				from = validation.ConvertToQAddress(from)
			}
			to := call.To
			if to != "" {
				if err := validation.ValidateAddress(to); err != nil {
					zap.L().Warn("Invalid to address in internal call frame", zap.Error(err))
				}
				to = validation.ConvertToQAddress(to)
			}

			out = append(out, InternalCall{
				Type:         call.Type,
				From:         from,
				To:           to,
				Input:        hexOrDefault(call.Input, "0x"),
				Output:       hexOrDefault(call.Output, "0x"),
				TraceAddress: tracePath,
				Value:        valueFloat,
				Gas:          hexOrDefault(call.Gas, "0x0"),
				GasUsed:      hexOrDefault(call.GasUsed, "0x0"),
			})
		}

		out = append(out, flattenCalls(call.Calls, tracePath)...)
	}
	return out
}

// emptyTrace returns a zero-value DebugTraceResult, optionally carrying an error.
func emptyTrace(err error) DebugTraceResult {
	return DebugTraceResult{Err: err}
}

// CallDebugTraceTransaction calls debug_traceTransaction and returns the parsed
// result as a DebugTraceResult struct.
// On Testnet V2+, debug_ APIs are not available. Set ENABLE_DEBUG_TRACE=true to
// re-enable if the node supports it.
func CallDebugTraceTransaction(hash string) DebugTraceResult {
	// Skip debug tracing when disabled (default for V2 nodes without debug_ API)
	if os.Getenv("ENABLE_DEBUG_TRACE") != "true" {
		return emptyTrace(nil)
	}

	// Validate transaction hash
	if err := validation.ValidateHexString(hash, validation.HashLength); err != nil {
		zap.L().Error("Invalid transaction hash", zap.Error(err))
		return emptyTrace(err)
	}

	var tracerResponse models.TraceResponse

	tracerOption := map[string]string{
		"tracer": "callTracer",
	}
	params := []interface{}{hash, tracerOption}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "debug_traceTransaction",
		Params:  params,
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Error("Failed JSON marshal", zap.Error(err))
		return emptyTrace(err)
	}

	// debug_traceTransaction is a primary-only method, public/foundation nodes
	// don't expose the debug_ namespace. Pin to the primary URL so we don't
	// surface a misleading "method not found" by failing over.
	primary := Endpoints().PrimaryURL()
	if primary == "" {
		zap.L().Error("No node endpoint configured for debug trace")
		return emptyTrace(fmt.Errorf("no node endpoint configured"))
	}
	body, err := postWithRetry(context.Background(), primary, b, 1)
	if err != nil {
		zap.L().Error("Failed to execute request", zap.Error(err))
		return emptyTrace(err)
	}

	if err = json.Unmarshal(body, &tracerResponse); err != nil {
		zap.L().Error("Failed to unmarshal response", zap.Error(err))
		return emptyTrace(err)
	}

	// Surface JSON-RPC errors instead of treating them as an empty trace;
	// a non-archive node cannot trace old transactions ("historical state
	// not available") and callers must be able to tell that apart from a
	// transaction that genuinely has no internal calls.
	if tracerResponse.Error != nil {
		return emptyTrace(fmt.Errorf("debug_traceTransaction failed for %s: %s (code %d)",
			hash, tracerResponse.Error.Message, tracerResponse.Error.Code))
	}

	var res DebugTraceResult

	// Flatten the nested call frames before the legacy top-level gate below:
	// a transaction whose top-level frame carries no interesting call data
	// can still contain value-moving sub-frames (e.g. an HTLC claim paying
	// out contract-held funds). A reverted top-level frame voids the whole
	// tree, so nothing is collected in that case.
	if tracerResponse.Result.Error == "" {
		res.InternalCalls = flattenCalls(tracerResponse.Result.Calls, nil)
	}

	// Validate and parse gas values
	if tracerResponse.Result.Gas != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Gas) {
			zap.L().Error("Invalid gas format", zap.String("gas", tracerResponse.Result.Gas))
		} else if parsed, err := strconv.ParseUint(tracerResponse.Result.Gas[2:], 16, 64); err == nil {
			res.Gas = parsed
		} else {
			zap.L().Warn("Failed to parse gas value", zap.Error(err))
		}
	}

	if tracerResponse.Result.GasUsed != "" {
		if !validation.IsValidHexString(tracerResponse.Result.GasUsed) {
			zap.L().Error("Invalid gasUsed format", zap.String("gasUsed", tracerResponse.Result.GasUsed))
		} else if parsed, err := strconv.ParseUint(tracerResponse.Result.GasUsed[2:], 16, 64); err == nil {
			res.GasUsed = parsed
		} else {
			zap.L().Warn("Failed to parse gasUsed value", zap.Error(err))
		}
	}

	// Parse the value field
	if tracerResponse.Result.Value != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Value) {
			zap.L().Error("Invalid value format", zap.String("value", tracerResponse.Result.Value))
		} else {
			valueBigInt := new(big.Int)
			valueBigInt.SetString(tracerResponse.Result.Value[2:], 16)

			divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
			bigIntAsFloat := new(big.Float).SetInt(valueBigInt)
			resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
			valueFloat64, _ := resultBigFloat.Float64()
			res.Value = float32(valueFloat64)

			zap.L().Debug("Parsed transaction value",
				zap.String("hex_value", tracerResponse.Result.Value),
				zap.Float32("parsed_value", res.Value))
		}
	}

	// Check if we have valid call data
	hasValidCallData := (len(tracerResponse.Result.Calls) > 0 &&
		tracerResponse.Result.Calls[0].From != "" &&
		tracerResponse.Result.Output != "") ||
		(tracerResponse.Result.CallType == "delegatecall" &&
			tracerResponse.Result.Output != "") ||
		tracerResponse.Result.Type == "CALL"

	if !hasValidCallData {
		// Keep the flattened sub-frames even when the legacy top-level
		// fields stay empty; the caller persists them independently.
		return DebugTraceResult{InternalCalls: res.InternalCalls}
	}

	// Validate addresses and convert to Q format
	if tracerResponse.Result.From != "" {
		if err := validation.ValidateAddress(tracerResponse.Result.From); err != nil {
			zap.L().Error("Invalid from address", zap.Error(err))
		}
		res.From = validation.ConvertToQAddress(tracerResponse.Result.From)
	}

	if tracerResponse.Result.To != "" {
		if err := validation.ValidateAddress(tracerResponse.Result.To); err != nil {
			zap.L().Error("Invalid to address", zap.Error(err))
		}
		res.To = validation.ConvertToQAddress(tracerResponse.Result.To)
	}

	// Validate and process output
	res.Output = 1
	if tracerResponse.Result.Output != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Output) {
			zap.L().Error("Invalid output format", zap.String("output", tracerResponse.Result.Output))
			res.Output = 0
		} else if tracerResponse.Result.Output != "0x" && len(tracerResponse.Result.Output) > 2 {
			hexStr := strings.TrimPrefix(tracerResponse.Result.Output, "0x")
			hexStr = strings.TrimLeft(hexStr, "0")

			if len(tracerResponse.Result.Output) == 42 { // "0x" + 40 chars, an address
				res.Output = 1
			} else if hexStr == "" {
				res.Output = 0
			} else {
				if parsed, err := strconv.ParseUint(hexStr, 16, 64); err == nil {
					res.Output = parsed
				} else {
					zap.L().Debug("Output value too large for uint64, storing 1",
						zap.String("output", tracerResponse.Result.Output))
					res.Output = 1
				}
			}
		}
	}

	// Safely handle TraceAddress
	if tracerResponse.Result.TraceAddress != nil {
		res.TraceAddress = make([]int, len(tracerResponse.Result.TraceAddress))
		copy(res.TraceAddress, tracerResponse.Result.TraceAddress)
	}

	// Process input data if it exists and has sufficient length
	const prefixLength = 2
	const methodIDLength = 8
	const addressLength = 64
	const minimumLength = prefixLength + methodIDLength + addressLength

	if len(tracerResponse.Result.Input) > minimumLength {
		if !validation.IsValidHexString(tracerResponse.Result.Input) {
			zap.L().Error("Invalid input format", zap.String("input", tracerResponse.Result.Input))
		} else {
			// Strip the '0x' prefix and method ID (first 4 bytes = 8 hex chars)
			data := tracerResponse.Result.Input[10:]

			if len(data) >= 64 {
				extractedAddr := "0x" + data[24:64]
				if err := validation.ValidateAddress(extractedAddr); err == nil {
					res.AddressFunctionIdentifier = validation.ConvertToQAddress(extractedAddr)
				} else {
					zap.L().Error("Invalid extracted address", zap.Error(err))
				}

				if len(data) >= 128 {
					amountHex := data[64:128]
					if !validation.IsValidHexString("0x" + amountHex) {
						zap.L().Error("Invalid amount format", zap.String("amount_hex", amountHex))
					} else if amountBigInt := new(big.Int); func() bool {
						_, ok := amountBigInt.SetString(amountHex, 16)
						return ok
					}() {
						if amountBigInt.IsUint64() {
							res.AmountFunctionIdentifier = amountBigInt.Uint64()
						} else {
							zap.L().Warn("Amount exceeds uint64 range")
						}
					} else {
						zap.L().Warn("Failed to parse amount", zap.String("amount_hex", amountHex))
					}
				}
			}
		}
	}

	res.TransactionType = tracerResponse.Result.Type
	res.CallType = tracerResponse.Result.CallType
	// Input field is reserved but not populated in the current implementation.
	res.Input = 0

	return res
}

func GetBalance(address string) (string, error) {
	// Validate input address format
	if err := validation.ValidateAddress(address); err != nil {
		return "", fmt.Errorf("invalid address format: %v", err)
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := rpcCall("qrl_getBalance", []interface{}{address, "latest"}, &result); err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", err
	}

	// Validate balance format
	if !validation.IsValidHexString(result.Result) {
		return "", fmt.Errorf("invalid balance format in response: %s", result.Result)
	}

	return result.Result, nil
}

// GetValidators fetches up to maxPages of validators from the beacon chain
// API and returns the parsed pages. Persistence is the caller's concern
// (synchroniser feeds the pages to services.StoreValidators); keeping this
// function transport-only keeps rpc free of any db/services dependency.
func GetValidators() ([]models.BeaconValidatorResponse, error) {
	zap.L().Info("Starting GetValidators call to beacon chain API")

	beaconchainURL := os.Getenv("BEACONCHAIN_API")
	if beaconchainURL == "" {
		return nil, fmt.Errorf("BEACONCHAIN_API environment variable not set")
	}

	// Base URL for the validators endpoint
	baseURL := strings.TrimRight(beaconchainURL, "/") + "/qrl/v1alpha1/validators"
	client := GetHTTPClient()

	pageToken := ""
	maxPages := 3 // Configurable based on needs
	pages := make([]models.BeaconValidatorResponse, 0, maxPages)

	for len(pages) < maxPages {
		requestURL := baseURL
		if pageToken != "" {
			requestURL += "?page_token=" + pageToken
		}

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get response from beacon API: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code from beacon API: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}

		var beaconResponse models.BeaconValidatorResponse
		err = json.Unmarshal(body, &beaconResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %v", err)
		}

		pages = append(pages, beaconResponse)

		// Check if there's a next page
		if beaconResponse.NextPageToken == "" {
			break
		}
		pageToken = beaconResponse.NextPageToken
	}

	zap.L().Info("Completed fetching validators",
		zap.Int("pages_processed", len(pages)))

	return pages, nil
}

// GetBeaconChainHead fetches the current chain head information from the beacon chain API
func GetBeaconChainHead() (*models.BeaconChainHeadResponse, error) {
	beaconchainURL := os.Getenv("BEACONCHAIN_API")
	if beaconchainURL == "" {
		return nil, fmt.Errorf("BEACONCHAIN_API environment variable not set")
	}

	url := strings.TrimRight(beaconchainURL, "/") + "/qrl/v1alpha1/beacon/chainhead"
	client := GetHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from beacon API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from beacon API: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var chainHead models.BeaconChainHeadResponse
	if err := json.Unmarshal(body, &chainHead); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	zap.L().Debug("Fetched beacon chain head",
		zap.String("headEpoch", chainHead.HeadEpoch),
		zap.String("headSlot", chainHead.HeadSlot),
		zap.String("finalizedEpoch", chainHead.FinalizedEpoch))

	return &chainHead, nil
}

func GetCode(address string, blockNrOrHash string) (string, error) {
	// Validate address format
	if err := validation.ValidateAddress(address); err != nil {
		return "", fmt.Errorf("invalid address format: %v", err)
	}

	var GetCode models.GetCode
	if err := rpcCall("qrl_getCode", []interface{}{address, "latest"}, &GetCode); err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", err
	}

	// Validate response format
	if GetCode.Result != "" && !validation.IsValidHexString(GetCode.Result) {
		return "", fmt.Errorf("invalid code format in response: %s", GetCode.Result)
	}

	return GetCode.Result, nil
}

// ZondGetBlockLogs retrieves logs for a specific block with an optional
// topic[0] filter. When `topic0Filters` has >1 entry the values are
// OR-matched against topic[0], the standard JSON-RPC encoding is a
// nested array `[[t0a, t0b, ...]]` (the outer position is the topic index,
// the inner array is the candidate set for that position).
func ZondGetBlockLogs(blockNumber string, topic0Filters []string) (*models.ZondLogsResponse, error) {
	// Validate block number format - Zond uses 0x prefix for block numbers
	if len(blockNumber) == 0 || (!strings.HasPrefix(blockNumber, "0x") && blockNumber != "latest") {
		return nil, fmt.Errorf("invalid block number format: %s", blockNumber)
	}

	filter := map[string]interface{}{
		"fromBlock": blockNumber,
		"toBlock":   blockNumber,
	}

	if len(topic0Filters) > 0 {
		filter["topics"] = [][]string{topic0Filters}
	}

	var responseData models.ZondLogsResponse
	if err := rpcCall("qrl_getLogs", []interface{}{filter}, &responseData); err != nil {
		zap.L().Info("Failed to get response from RPC call", zap.Error(err))
		return nil, err
	}

	return &responseData, nil
}

// GetTxDetailsByHash retrieves transaction details by hash
func GetTxDetailsByHash(txHash string) (*models.TransactionResult, error) {
	zap.L().Info("Getting transaction details", zap.String("txHash", txHash))

	// Validate transaction hash
	if err := validation.ValidateHexString(txHash, validation.HashLength); err != nil {
		return nil, fmt.Errorf("invalid transaction hash: %v", err)
	}

	// rpcCall also surfaces the JSON-RPC error member: previously a node
	// error here was silently ignored and an empty Result returned.
	var tx models.TransactionResponse
	if err := rpcCall("qrl_getTransactionByHash", []interface{}{txHash}, &tx); err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return nil, err
	}

	return &tx.Result, nil
}
