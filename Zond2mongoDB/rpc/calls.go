package rpc

import (
	"Zond2mongoDB/configs"
	L "Zond2mongoDB/logger"
	"Zond2mongoDB/models"
	"bytes"
	"encoding/hex"
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

var logger *zap.Logger = L.FileLogger(configs.Filename)

func GetLatestBlock() (uint64, error) {
	var Zond models.RPC

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_blockNumber",
		Params:  []interface{}{},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return 0, err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to get response from RPC call", zap.Error(err))
		return 0, err
	}
	if resp == nil {
		return 0, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return 0, err
	}

	err = json.Unmarshal([]byte(string(body)), &Zond)
	if err != nil {
		logger.Info("Failed to unmarshal response", zap.Error(err))
		return 0, err
	}

	blockNumber := new(big.Int)
	blockNumber.SetString(Zond.Result[2:], 16)
	return blockNumber.Uint64(), nil
}

func GetBlockByNumberMainnet(blocknumber uint64) string {
	h := fmt.Sprintf("0x%x", blocknumber)

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getBlockByNumber",
		Params:  []interface{}{h, true},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return ""
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to get response from RPC call", zap.Error(err))
		return ""
	}
	if resp == nil {
		logger.Info("Received nil response")
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return ""
	}

	return string(body)
}

func GetContractAddress(txHash string) (string, string, error) {
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getTransactionReceipt",
		Params:  []interface{}{txHash},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return "", "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to execute request", zap.Error(err))
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return "", "", err
	}

	var ContractAddress models.Contract
	err = json.Unmarshal([]byte(string(body)), &ContractAddress)
	if err != nil {
		logger.Info("Failed to unmarshal response", zap.Error(err))
		return "", "", err
	}

	return ContractAddress.Result.ContractAddress, ContractAddress.Result.Status, nil
}

func CallDebugTraceTransaction(hash string) (transactionType []byte, callType []byte, from []byte, to []byte, input uint64, output uint64, traceAddress []int, value float32, gas uint64, gasUsed uint64, addressFunctionidentifier []byte, amountFunctionIdentifier uint64) {
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
		logger.Error("Failed JSON marshal", zap.Error(err))
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, 0, nil, 0
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Error("Failed to create request", zap.Error(err))
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, 0, nil, 0
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to execute request", zap.Error(err))
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, 0, nil, 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", zap.Error(err))
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, 0, nil, 0
	}

	err = json.Unmarshal([]byte(string(body)), &tracerResponse)
	if err != nil {
		logger.Error("Failed to unmarshal response", zap.Error(err))
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, 0, nil, 0
	}

	// Initialize default values for gas and gasUsed
	gas = 0
	gasUsed = 0

	// Safely parse gas and gasUsed if they exist
	if tracerResponse.Result.Gas != "" {
		gas, err = strconv.ParseUint(tracerResponse.Result.Gas[2:], 16, 64)
		if err != nil {
			logger.Warn("Failed to parse gas value", zap.Error(err))
		}
	}

	if tracerResponse.Result.GasUsed != "" {
		gasUsed, err = strconv.ParseUint(tracerResponse.Result.GasUsed[2:], 16, 64)
		if err != nil {
			logger.Warn("Failed to parse gasUsed value", zap.Error(err))
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
		return nil, nil, nil, nil, 0, 0, nil, 0, 0, gasUsed, nil, 0
	}

	// Process From and To addresses
	from, err = hex.DecodeString(tracerResponse.Result.From[2:])
	if err != nil {
		logger.Warn("Failed to decode From address", zap.Error(err))
		from = []byte{}
	}

	to, err = hex.DecodeString(tracerResponse.Result.To[2:])
	if err != nil {
		logger.Warn("Failed to decode To address", zap.Error(err))
		to = []byte{}
	}

	// Process output
	output = 1
	if tracerResponse.Result.Output != "0x" && len(tracerResponse.Result.Output) > 2 {
		output, err = strconv.ParseUint(tracerResponse.Result.Output[2:], 16, 64)
		if err != nil {
			logger.Warn("Failed to parse output value", zap.Error(err))
			output = 0
		}
	}

	// Process value
	bigInt := new(big.Int)
	if tracerResponse.Result.Value != "" {
		_, ok := bigInt.SetString(tracerResponse.Result.Value[2:], 16)
		if !ok {
			logger.Warn("Failed to parse value")
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

	addressFunctionidentifier = nil
	amountFunctionIdentifier = 0

	if len(tracerResponse.Result.Input) > minimumLength {
		// Strip the '0x' prefix and method ID
		data := tracerResponse.Result.Input[10:]

		// Extract address (next 64 characters, but only last 40 are significant)
		if len(data) >= 64 {
			addressHex := data[24:64]
			addressFunctionidentifier, err = hex.DecodeString(addressHex)
			if err != nil {
				logger.Warn("Failed to decode address from input", zap.Error(err))
			}

			// Extract amount if there's enough data
			if len(data) >= 128 {
				amountHex := data[64:128]
				amountBytes, err := hex.DecodeString(amountHex)
				if err != nil {
					logger.Warn("Failed to decode amount from input", zap.Error(err))
				} else {
					amountBigInt := new(big.Int).SetBytes(amountBytes)
					amountFunctionIdentifier = amountBigInt.Uint64()
				}
			}
		}
	}

	return []byte(tracerResponse.Result.Type),
		[]byte(tracerResponse.Result.CallType),
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
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getBalance",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to execute request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return "", err
	}

	return string(body), nil
}

func GetValidators() models.ResultValidator {
	logger.Info("Starting GetValidators call to beacon chain API")
	var allValidators models.ResultValidator
	pageToken := ""
	maxPages := 3
	currentPage := 0
	totalValidators := 0

	beaconchainURL := os.Getenv("BEACONCHAIN_API")
	if beaconchainURL == "" {
		logger.Error("BEACONCHAIN_API environment variable not set")
		return models.ResultValidator{}
	}

	// Base URL for the validators endpoint
	baseURL := strings.TrimRight(beaconchainURL, "/") + "/zond/v1alpha1/validators"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Initialize the result
	allValidators.ValidatorsBySlotNumber = make([]models.ValidatorsBySlotNumber, 0)
	validatorMap := make(map[int][]string) // Map to accumulate validators across pages

	for currentPage < maxPages {
		requestURL := baseURL
		if pageToken != "" {
			requestURL += "?page_token=" + pageToken
		}

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			logger.Error("Failed to create request", zap.Error(err))
			break
		}
		logger.Info("Created HTTP request for validators",
			zap.String("url", requestURL),
			zap.Int("page", currentPage+1),
			zap.Int("maxPages", maxPages))

		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Failed to get response from beacon API", zap.Error(err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			logger.Error("Unexpected status code from beacon API",
				zap.Int("status_code", resp.StatusCode))
			resp.Body.Close()
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			break
		}

		var beaconResponse models.BeaconValidatorResponse
		err = json.Unmarshal(body, &beaconResponse)
		if err != nil {
			logger.Error("Failed to unmarshal response", zap.Error(err))
			logger.Error("Response body:", zap.String("body", string(body)))
			break
		}

		// Update total validators count
		if currentPage == 0 {
			totalValidators = beaconResponse.TotalSize
			logger.Info("Total validators reported by API", zap.Int("total_validators", totalValidators))
		}

		// Process validators from this page
		for _, validator := range beaconResponse.ValidatorList {
			index, err := strconv.Atoi(validator.Index)
			if err != nil {
				logger.Warn("Failed to parse validator index",
					zap.String("index", validator.Index),
					zap.Error(err))
				continue
			}
			slotNumber := index % 100 // Assuming 100 slots per epoch
			validatorMap[slotNumber] = append(validatorMap[slotNumber], validator.Validator.PublicKey)
		}

		logger.Info("Processed validator page",
			zap.Int("page", currentPage+1),
			zap.Int("validators_on_page", len(beaconResponse.ValidatorList)),
			zap.String("next_page_token", beaconResponse.NextPageToken))

		currentPage++

		// Check if there's a next page
		if beaconResponse.NextPageToken == "" {
			logger.Info("No more pages available", zap.Int("total_pages_fetched", currentPage))
			break
		}
		pageToken = beaconResponse.NextPageToken
	}

	// Convert accumulated map to ValidatorsBySlotNumber slice
	for slotNumber, validators := range validatorMap {
		if len(validators) > 0 {
			slotValidators := models.ValidatorsBySlotNumber{
				SlotNumber: slotNumber,
				Leader:     validators[0],
				Attestors:  validators[1:],
			}
			allValidators.ValidatorsBySlotNumber = append(allValidators.ValidatorsBySlotNumber, slotValidators)
		}
	}

	logger.Info("Completed fetching all validators",
		zap.Int("total_pages", currentPage),
		zap.Int("total_validators_processed", len(validatorMap)),
		zap.Int("total_slots", len(allValidators.ValidatorsBySlotNumber)),
		zap.Int("expected_total", totalValidators))

	return allValidators
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetCode(address string, blockNrOrHash string) (string, error) {
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getCode",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to execute request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return "", err
	}

	var GetCode models.GetCode
	err = json.Unmarshal([]byte(string(body)), &GetCode)
	if err != nil {
		logger.Info("Failed to unmarshal response", zap.Error(err))
		return "", err
	}

	return GetCode.Result, nil
}

func ZondCall(contractAddress string) *models.ZondResponse {
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
		logger.Info("Failed JSON marshal", zap.Error(err))
		return nil
	}

	resp, err := http.Post(os.Getenv("NODE_URL"), "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Info("Failed to get a response from HTTP POST request", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return nil
	}

	var responseData models.ZondResponse
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		logger.Info("Failed JSON unmarshal", zap.Error(err))
		return nil
	}

	return &responseData
}

func ZondGetLogs(contractAddress string) *models.ZondResponse {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "zond_getLogs",
		"params":  []string{contractAddress},
		"id":      1,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		logger.Info("Failed JSON marshal", zap.Error(err))
		return nil
	}

	resp, err := http.Post(os.Getenv("NODE_URL"), "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Info("Failed to get a response from HTTP POST request", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return nil
	}

	var responseData models.ZondResponse
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		logger.Info("Failed JSON unmarshal", zap.Error(err))
		return nil
	}

	return &responseData
}

// Common ERC20 function signatures
const (
	SIG_NAME     = "0x06fdde03" // name()
	SIG_SYMBOL   = "0x95d89b41" // symbol()
	SIG_DECIMALS = "0x313ce567" // decimals()
)

// CallContractMethod makes a zond_call to a contract method and returns the result
func CallContractMethod(contractAddress string, methodSig string) (string, error) {
	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_call",
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
		logger.Info("Failed JSON marshal", zap.Error(err))
		return "", err
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	if err != nil {
		logger.Info("Failed to create request", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to get response from RPC call", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Info("Failed to read response body", zap.Error(err))
		return "", err
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Info("Failed to unmarshal response", zap.Error(err))
		return "", err
	}

	return result.Result, nil
}

// GetTokenInfo attempts to get ERC20 token information for a contract
func GetTokenInfo(contractAddress string) (name string, symbol string, decimals uint8, isToken bool) {
	logger.Info("Getting token info", zap.String("contract_address", contractAddress))

	name, err := getTokenName(contractAddress)
	if err != nil {
		logger.Info("Failed to get token name",
			zap.String("contract_address", contractAddress),
			zap.Error(err))
		name = ""
	} else {
		logger.Info("Got token name",
			zap.String("contract_address", contractAddress),
			zap.String("name", name))
	}

	symbol, err = getTokenSymbol(contractAddress)
	if err != nil {
		logger.Info("Failed to get token symbol",
			zap.String("contract_address", contractAddress),
			zap.Error(err))
		symbol = ""
	} else {
		logger.Info("Got token symbol",
			zap.String("contract_address", contractAddress),
			zap.String("symbol", symbol))
	}

	decimals, err = getTokenDecimals(contractAddress)
	if err != nil {
		logger.Info("Failed to get token decimals",
			zap.String("contract_address", contractAddress),
			zap.Error(err))
		decimals = 0
	} else {
		logger.Info("Got token decimals",
			zap.String("contract_address", contractAddress),
			zap.Uint8("decimals", decimals))
	}

	// If we got here, all token checks passed
	logger.Info("Successfully identified token",
		zap.String("contract_address", contractAddress))
	return name, symbol, decimals, true
}

func getTokenName(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_NAME)
	if err != nil {
		return "", err
	}

	nameHex := strings.TrimRight(result[66:], "0")
	if decoded, err := hex.DecodeString(nameHex); err != nil {
		return "", err
	} else {
		return string(decoded), nil
	}
}

func getTokenSymbol(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SYMBOL)
	if err != nil {
		return "", err
	}

	symbolHex := strings.TrimRight(result[66:], "0")
	if decoded, err := hex.DecodeString(symbolHex); err != nil {
		return "", err
	} else {
		return string(decoded), nil
	}
}

func getTokenDecimals(contractAddress string) (uint8, error) {
	result, err := CallContractMethod(contractAddress, SIG_DECIMALS)
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseUint(result[2:], 16, 8)
	if err != nil {
		return 0, err
	}

	return uint8(val), nil
}
