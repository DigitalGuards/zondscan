package rpc

import (
	"Zond2mongoDB/models"
	"Zond2mongoDB/services"
	"Zond2mongoDB/utils"
	"Zond2mongoDB/validation"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

func GetLatestBlock() (string, error) {
	var Zond models.RPC

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_blockNumber",
		Params:  []interface{}{},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return "0x0", err
	}

	// Retry logic with exponential backoff
	var resp *http.Response
	var lastErr error
	for retries := 0; retries < 3; retries++ {
		req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = GetHTTPClient().Do(req)
		if err == nil && resp != nil {
			break
		}

		lastErr = err
		if resp != nil {
			resp.Body.Close()
		}

		// Exponential backoff
		backoffDuration := time.Duration(1<<uint(retries)) * time.Second
		zap.L().Warn("RPC call failed, retrying...",
			zap.Error(err),
			zap.Int("retry", retries+1),
			zap.Duration("backoff", backoffDuration))
		time.Sleep(backoffDuration)
	}

	if lastErr != nil {
		zap.L().Error("Failed to get response from RPC call after retries", zap.Error(lastErr))
		return "0x0", lastErr
	}
	if resp == nil {
		return "0x0", fmt.Errorf("received nil response after retries")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return "0x0", err
	}

	err = json.Unmarshal([]byte(string(body)), &Zond)
	if err != nil {
		zap.L().Info("Failed to unmarshal response", zap.Error(err))
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

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getBlockByNumber",
		Params:  []interface{}{blockNumber, true},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return nil, err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Info("Failed to create request", zap.Error(err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Info("Failed to get response from RPC call", zap.Error(err))
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return nil, err
	}

	var block models.ZondDatabaseBlock
	if err := json.Unmarshal(body, &block); err != nil {
		zap.L().Info("Failed to unmarshal block", zap.Error(err))
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
			if err := validation.ValidateHexString(tx.To, validation.AddressLength); err != nil {
				return nil, fmt.Errorf("invalid to address: %v", err)
			}
		}
		if err := validation.ValidateHexString(tx.From, validation.AddressLength); err != nil {
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
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getTransactionReceipt",
		Params:  []interface{}{txHash},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return "", "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Info("Failed to create request", zap.Error(err))
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return "", "", err
	}

	var ContractAddress models.Contract
	err = json.Unmarshal([]byte(string(body)), &ContractAddress)
	if err != nil {
		zap.L().Info("Failed to unmarshal response", zap.Error(err))
		return "", "", err
	}

	// Validate contract address if present
	if ContractAddress.Result.ContractAddress != "" {
		if err := validation.ValidateHexString(ContractAddress.Result.ContractAddress, validation.AddressLength); err != nil {
			return "", "", fmt.Errorf("invalid contract address in response: %v", err)
		}
	}

	// Validate status format
	if ContractAddress.Result.Status != "" && !validation.IsValidHexString(ContractAddress.Result.Status) {
		return "", "", fmt.Errorf("invalid status format in response: %s", ContractAddress.Result.Status)
	}

	return ContractAddress.Result.ContractAddress, ContractAddress.Result.Status, nil
}

func CallDebugTraceTransaction(hash string) (transactionType string, callType string, from string, to string, input uint64, output uint64, traceAddress []int, value float32, gas uint64, gasUsed uint64, addressFunctionidentifier string, amountFunctionIdentifier uint64) {
	// Validate transaction hash
	if err := validation.ValidateHexString(hash, validation.HashLength); err != nil {
		zap.L().Error("Invalid transaction hash", zap.Error(err))
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
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
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Error("Failed to create request", zap.Error(err))
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Error("Failed to execute request", zap.Error(err))
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("Failed to read response body", zap.Error(err))
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}

	err = json.Unmarshal([]byte(string(body)), &tracerResponse)
	if err != nil {
		zap.L().Error("Failed to unmarshal response", zap.Error(err))
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}

	// Initialize default values for gas and gasUsed
	gas = 0
	gasUsed = 0

	// Validate and parse gas values
	if tracerResponse.Result.Gas != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Gas) {
			zap.L().Error("Invalid gas format", zap.String("gas", tracerResponse.Result.Gas))
		} else if parsed, err := strconv.ParseUint(tracerResponse.Result.Gas[2:], 16, 64); err == nil {
			gas = parsed
		} else {
			zap.L().Warn("Failed to parse gas value", zap.Error(err))
		}
	}

	if tracerResponse.Result.GasUsed != "" {
		if !validation.IsValidHexString(tracerResponse.Result.GasUsed) {
			zap.L().Error("Invalid gasUsed format", zap.String("gasUsed", tracerResponse.Result.GasUsed))
		} else if parsed, err := strconv.ParseUint(tracerResponse.Result.GasUsed[2:], 16, 64); err == nil {
			gasUsed = parsed
		} else {
			zap.L().Warn("Failed to parse gasUsed value", zap.Error(err))
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
		return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
	}

	// Validate addresses
	if tracerResponse.Result.From != "" {
		if err := validation.ValidateHexString(tracerResponse.Result.From, validation.AddressLength); err != nil {
			zap.L().Error("Invalid from address", zap.Error(err))
			return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
		}
		from = tracerResponse.Result.From
	}

	if tracerResponse.Result.To != "" {
		if err := validation.ValidateHexString(tracerResponse.Result.To, validation.AddressLength); err != nil {
			zap.L().Error("Invalid to address", zap.Error(err))
			return "", "", "", "", 0, 0, nil, 0, 0, 0, "", 0
		}
		to = tracerResponse.Result.To
	}

	// Validate and process output
	output = 1
	if tracerResponse.Result.Output != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Output) {
			zap.L().Error("Invalid output format", zap.String("output", tracerResponse.Result.Output))
			output = 0
		} else if tracerResponse.Result.Output != "0x" && len(tracerResponse.Result.Output) > 2 {
			// Remove "0x" prefix and leading zeros
			hexStr := strings.TrimPrefix(tracerResponse.Result.Output, "0x")
			hexStr = strings.TrimLeft(hexStr, "0")
			
			// If it's an address (40 characters), just store 1 to indicate success
			if len(tracerResponse.Result.Output) == 42 { // "0x" + 40 chars
				output = 1
			} else if hexStr == "" {
				output = 0
			} else {
				// Try to parse as uint64 if it's a small enough number
				if parsed, err := strconv.ParseUint(hexStr, 16, 64); err == nil {
					output = parsed
				} else {
					// For larger numbers, just store 1 to indicate success
					zap.L().Debug("Output value too large for uint64, storing 1", 
						zap.String("output", tracerResponse.Result.Output))
					output = 1
				}
			}
		}
	}

	// Validate and process value
	if tracerResponse.Result.Value != "" {
		if !validation.IsValidHexString(tracerResponse.Result.Value) {
			zap.L().Error("Invalid value format", zap.String("value", tracerResponse.Result.Value))
		} else {
			// Remove "0x" prefix and leading zeros
			hexStr := strings.TrimPrefix(tracerResponse.Result.Value, "0x")
			hexStr = strings.TrimLeft(hexStr, "0")
			
			bigInt := new(big.Int)
			if _, ok := bigInt.SetString(hexStr, 16); !ok {
				zap.L().Warn("Failed to parse value")
			}
		}
	}

	// Safely handle TraceAddress
	traceAddress = nil
	if tracerResponse.Result.TraceAddress != nil {
		traceAddress = make([]int, len(tracerResponse.Result.TraceAddress))
		copy(traceAddress, tracerResponse.Result.TraceAddress)
	}

	// Process input data if it exists and has sufficient length
	const prefixLength = 2
	const methodIDLength = 8
	const addressLength = 64
	const minimumLength = prefixLength + methodIDLength + addressLength

	addressFunctionidentifier = ""
	amountFunctionIdentifier = 0

	if len(tracerResponse.Result.Input) > minimumLength {
		// Validate input format
		if !validation.IsValidHexString(tracerResponse.Result.Input) {
			zap.L().Error("Invalid input format", zap.String("input", tracerResponse.Result.Input))
		} else {
			// Strip the '0x' prefix and method ID
			data := tracerResponse.Result.Input[10:]

			// Extract and validate address
			if len(data) >= 64 {
				extractedAddr := "0x" + data[24:64]
				if err := validation.ValidateHexString(extractedAddr, validation.AddressLength); err == nil {
					addressFunctionidentifier = extractedAddr
				} else {
					zap.L().Error("Invalid extracted address", zap.Error(err))
				}

				// Extract and validate amount
				if len(data) >= 128 {
					amountHex := data[64:128]
					if !validation.IsValidHexString("0x" + amountHex) {
						zap.L().Error("Invalid amount format", zap.String("amount_hex", amountHex))
					} else if amountBigInt := new(big.Int); func() bool {
						_, ok := amountBigInt.SetString(amountHex, 16)
						return ok
					}() {
						if amountBigInt.IsUint64() {
							amountFunctionIdentifier = amountBigInt.Uint64()
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

	return tracerResponse.Result.Type,
		tracerResponse.Result.CallType,
		from,
		to,
		0, // input is not used in the current implementation
		output,
		traceAddress,
		0, // value is not used in the current implementation
		gas,
		gasUsed,
		addressFunctionidentifier,
		amountFunctionIdentifier
}

func GetBalance(address string) (string, error) {
	// Validate input address format
	if err := validation.ValidateHexString(address, validation.AddressLength); err != nil {
		return "", fmt.Errorf("invalid address format: %v", err)
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getBalance",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Info("Failed to create request", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return "", err
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Info("Failed to unmarshal response", zap.Error(err))
		return "", err
	}

	// Validate balance format
	if !validation.IsValidHexString(result.Result) {
		return "", fmt.Errorf("invalid balance format in response: %s", result.Result)
	}

	return result.Result, nil
}

func GetValidators() error {
	zap.L().Info("Starting GetValidators call to beacon chain API")

	beaconchainURL := os.Getenv("BEACONCHAIN_API")
	if beaconchainURL == "" {
		return fmt.Errorf("BEACONCHAIN_API environment variable not set")
	}

	// Base URL for the validators endpoint
	baseURL := strings.TrimRight(beaconchainURL, "/") + "/zond/v1alpha1/validators"
	client := GetHTTPClient()

	// Get current epoch from latest block
	latestBlock, err := GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := strconv.FormatUint(uint64(utils.HexToInt(latestBlock).Int64()/128), 10)

	pageToken := ""
	maxPages := 3 // Configurable based on needs
	currentPage := 0

	for currentPage < maxPages {
		requestURL := baseURL
		if pageToken != "" {
			requestURL += "?page_token=" + pageToken
		}

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get response from beacon API: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("unexpected status code from beacon API: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %v", err)
		}

		var beaconResponse models.BeaconValidatorResponse
		err = json.Unmarshal(body, &beaconResponse)
		if err != nil {
			return fmt.Errorf("failed to unmarshal response: %v", err)
		}

		// Store this page of validators using the validator service
		err = services.StoreValidators(beaconResponse, currentEpoch)
		if err != nil {
			return fmt.Errorf("failed to store validators: %v", err)
		}

		currentPage++

		// Check if there's a next page
		if beaconResponse.NextPageToken == "" {
			break
		}
		pageToken = beaconResponse.NextPageToken
	}

	zap.L().Info("Completed fetching validators",
		zap.Int("pages_processed", currentPage),
		zap.String("current_epoch", currentEpoch))

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetCode(address string, blockNrOrHash string) (string, error) {
	// Validate address format
	if err := validation.ValidateHexString(address, validation.AddressLength); err != nil {
		return "", fmt.Errorf("invalid address format: %v", err)
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getCode",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Info("Failed to create request", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Info("Failed to execute request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return "", err
	}

	var GetCode models.GetCode
	err = json.Unmarshal([]byte(string(body)), &GetCode)
	if err != nil {
		zap.L().Info("Failed to unmarshal response", zap.Error(err))
		return "", err
	}

	// Validate response format
	if GetCode.Result != "" && !validation.IsValidHexString(GetCode.Result) {
		return "", fmt.Errorf("invalid code format in response: %s", GetCode.Result)
	}

	return GetCode.Result, nil
}

func ZondCall(contractAddress string) (*models.ZondResponse, error) {
	// Validate contract address format
	if err := validation.ValidateHexString(contractAddress, validation.AddressLength); err != nil {
		return nil, fmt.Errorf("invalid contract address format: %v", err)
	}

	data := map[string]interface{}{
		"from":     "0x20748ad4e06597dbca756e2731cd26094c05273a",
		"chainId":  "0x0",
		"nonce":    "0x0",
		"gas":      "0x61184",
		"gasPrice": "0x2710",
		"to":       contractAddress,
		"value":    "0x0",
		"data":     "",
	}
	blockData := map[string]string{
		"blockNumber": "latest",
	}

	payload := models.ZondCallPayload{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "zond_call",
		Params:  []interface{}{data, blockData},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return nil, err
	}

	resp, err := GetHTTPClient().Post(os.Getenv("NODE_URL"), "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		zap.L().Info("Failed to get a response from HTTP POST request", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return nil, err
	}

	var responseData models.ZondResponse
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		zap.L().Info("Failed JSON unmarshal", zap.Error(err))
		return nil, err
	}

	// Validate response data
	if responseData.Result != "" && !validation.IsValidHexString(responseData.Result) {
		return nil, fmt.Errorf("invalid response format: %s", responseData.Result)
	}

	return &responseData, nil
}

func ZondGetLogs(contractAddress string) (*models.ZondResponse, error) {
	// Validate contract address format
	if err := validation.ValidateHexString(contractAddress, validation.AddressLength); err != nil {
		return nil, fmt.Errorf("invalid contract address: %v", err)
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getLogs",
		Params: []interface{}{
			map[string]interface{}{
				"address": contractAddress,
				"topics":  []string{},
			},
		},
		ID: 1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Info("Failed JSON marshal", zap.Error(err))
		return nil, err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		zap.L().Info("Failed to create request", zap.Error(err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		zap.L().Info("Failed to get response from RPC call", zap.Error(err))
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Info("Failed to read response body", zap.Error(err))
		return nil, err
	}

	var responseData models.ZondResponse
	err = json.Unmarshal([]byte(string(body)), &responseData)
	if err != nil {
		zap.L().Info("Failed to unmarshal response", zap.Error(err))
		return nil, err
	}

	return &responseData, nil
}
