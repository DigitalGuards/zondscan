package rpc

import (
	"QRLtoMongoDB-PoS/configs"
	L "QRLtoMongoDB-PoS/logger"
	"QRLtoMongoDB-PoS/models"
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
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var ContractAddress models.Contract

	json.Unmarshal([]byte(string(body)), &ContractAddress)

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
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return string(body), nil
}

func GetValidators() models.ResultValidator {
	logger.Info("Starting GetValidators call to beacon chain API")
	var beaconResponse models.BeaconValidatorResponse

	beaconchainURL := os.Getenv("BEACONCHAIN_API")
	if beaconchainURL == "" {
		logger.Error("BEACONCHAIN_API environment variable not set")
		return models.ResultValidator{}
	}

	req, err := http.NewRequest("GET", beaconchainURL, nil)
	if err != nil {
		logger.Error("Failed to create request", zap.Error(err))
		return models.ResultValidator{}
	}
	logger.Info("Created HTTP request for validators")

	client := &http.Client{
		Timeout: 30 * time.Second, // Increased timeout as the response is large
	}
	logger.Info("Sending request to beacon chain API")
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to get response from beacon API", zap.Error(err))
		return models.ResultValidator{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Unexpected status code from beacon API",
			zap.Int("status_code", resp.StatusCode))
		return models.ResultValidator{}
	}

	logger.Info("Got response from beacon chain API",
		zap.Int("status_code", resp.StatusCode),
		zap.String("content_type", resp.Header.Get("Content-Type")))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", zap.Error(err))
		return models.ResultValidator{}
	}

	if len(body) == 0 {
		logger.Error("Empty response body from beacon API")
		return models.ResultValidator{}
	}

	logger.Info("Received response body",
		zap.Int("body_length", len(body)))

	err = json.Unmarshal(body, &beaconResponse)
	if err != nil {
		logger.Error("Failed to unmarshal response",
			zap.Error(err),
			zap.String("body_preview", string(body[:min(len(body), 200)]))) // Log first 200 chars only
		return models.ResultValidator{}
	}

	if len(beaconResponse.ValidatorList) == 0 {
		logger.Warn("No validators found in response")
		return models.ResultValidator{}
	}

	logger.Info("Successfully unmarshaled response",
		zap.String("epoch", beaconResponse.Epoch),
		zap.Int("validator_count", len(beaconResponse.ValidatorList)))

	// Convert beacon response to ResultValidator format
	epoch, err := strconv.Atoi(beaconResponse.Epoch)
	if err != nil {
		logger.Error("Failed to parse epoch",
			zap.String("epoch", beaconResponse.Epoch),
			zap.Error(err))
		return models.ResultValidator{}
	}

	result := models.ResultValidator{
		Epoch:                  epoch,
		ValidatorsBySlotNumber: make([]models.ValidatorsBySlotNumber, 0),
	}

	// Group validators by slot number
	validatorMap := make(map[int][]string)
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

	// Convert map to ValidatorsBySlotNumber slice
	for slotNumber, validators := range validatorMap {
		if len(validators) > 0 {
			slotValidators := models.ValidatorsBySlotNumber{
				SlotNumber: slotNumber,
				Leader:     validators[0],
				Attestors:  validators[1:],
			}
			result.ValidatorsBySlotNumber = append(result.ValidatorsBySlotNumber, slotValidators)
		}
	}

	logger.Info("Finished processing validators",
		zap.Int("epoch", result.Epoch),
		zap.Int("slot_count", len(result.ValidatorsBySlotNumber)),
		zap.Int("total_validators", len(beaconResponse.ValidatorList)))

	return result
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
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var GetCode models.GetCode

	json.Unmarshal([]byte(string(body)), &GetCode)

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

	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post(os.Getenv("NODE_URL"), "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Info("Failed to get a response from HTTP POST request", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

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

	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post(os.Getenv("NODE_URL"), "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Info("Failed to get a response from HTTP POST request", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var responseData models.ZondResponse
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		logger.Info("Failed JSON unmarshal", zap.Error(err))
		return nil
	}

	return &responseData
}
