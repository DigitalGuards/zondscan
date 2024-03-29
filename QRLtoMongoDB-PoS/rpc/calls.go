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
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to get response from RPC call", zap.Error(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	err = json.Unmarshal([]byte(string(body)), &Zond)

	blockNumber := new(big.Int)
	blockNumber.SetString(Zond.Result[2:], 16)
	blockNumber.Uint64()

	return blockNumber.Uint64(), nil
}

// func GetLatestBlock(client *mock_rpc.MockMyHTTPClient) (uint64, error) {
// 	// define the parameters for the RPC call
// 	params := []interface{}{"latest", true}

// 	// create the request for the RPC call
// 	request := models.JsonRPC{
// 		Jsonrpc: "2.0",
// 		Method:  "zond_getBlockByNumber",
// 		Params:  params,
// 		ID:      1,
// 	}

// 	// marshal the request to JSON
// 	b, err := json.Marshal(request)
// 	if err != nil {
// 		return 0, fmt.Errorf("error marshalling request: %v", err)
// 	}

// 	// create a new HTTP request with the RPC request as the body
// 	httpRequest, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
// 	if err != nil {
// 		return 0, fmt.Errorf("error creating HTTP request: %v", err)
// 	}
// 	httpRequest.Header.Set("Content-Type", "application/json")

// 	// make the RPC call using the HTTP client
// 	resp, err := client.Do(httpRequest)
// 	if err != nil {
// 		return 0, fmt.Errorf("error making RPC call: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	// read the response body
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return 0, fmt.Errorf("error reading response body: %v", err)
// 	}

// 	// unmarshal the response to a Zond struct
// 	var zond models.Zond
// 	if err := json.Unmarshal([]byte(string(body)), &zond); err != nil {
// 		return 0, fmt.Errorf("error unmarshalling response: %v", err)
// 	}

// 	fmt.Println(zond.ResultOld.Number)

// 	// convert the block
// 	// convert the block number from hexadecimal to uint64
// 	blockNumber := new(big.Int)
// 	blockNumber.SetString(zond.ResultOld.Number[2:], 16)

// 	return blockNumber.Uint64(), nil
// }

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
	}

	req, err := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Failed to get response from RPC call", zap.Error(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return string(body)
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
		logger.Info("Failed JSON marshal", zap.Error(err))
	}

	req, _ := http.NewRequest("POST", os.Getenv("NODE_URL"), bytes.NewBuffer([]byte(b)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	err = json.Unmarshal([]byte(string(body)), &tracerResponse)

	gas, err = strconv.ParseUint(tracerResponse.Result.Gas[2:], 16, 64)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	gasUsed, err = strconv.ParseUint(tracerResponse.Result.GasUsed[2:], 16, 64)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	if len(tracerResponse.Result.Calls) > 0 && tracerResponse.Result.Calls[0].From != "" && tracerResponse.Result.Output != "" || tracerResponse.Result.CallType == "delegatecall" && tracerResponse.Result.Output != "" || tracerResponse.Result.Type == "CALL" {
		var output uint64

		from, err := hex.DecodeString(tracerResponse.Result.From[2:])
		if err != nil {
			configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
		}

		to, err := hex.DecodeString(tracerResponse.Result.To[2:])
		if err != nil {
			configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
		}

		if tracerResponse.Result.Output != "0x" && len(tracerResponse.Result.Output) > 2 {
			output, err = strconv.ParseUint(tracerResponse.Result.Output[2:], 16, 64)
			if err != nil {
				fmt.Println(tracerResponse.Result.Output)
				configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
			}
		} else {
			output = 1
		}

		// divisor := new(big.Float).SetFloat64(float64(configs.QUANTA))

		bigInt := new(big.Int)
		_, ok := bigInt.SetString(tracerResponse.Result.Value[2:], 16)
		if !ok {
			fmt.Println("Failed to parse hexadecimal string.")
		}

		// bigFloat := new(big.Float).SetInt(bigInt)

		// // result := new(big.Float).Quo(bigFloat, divisor)

		// value, _ := result.Float64()

		const prefixLength = 2
		const methodIDLength = 8
		const addressLength = 64
		const minimumLength = prefixLength + methodIDLength + addressLength

		if len(tracerResponse.Result.Input) > minimumLength {

			// Strip the '0x' prefix and method ID (first 10 bytes, or 20 characters in the string)
			data := tracerResponse.Result.Input[10:]

			// The next 64 characters are the address, padded to 32 bytes, but only the last 40 characters represent the address
			addressHex := data[24:64]
			addressFunctionidentifier, err := hex.DecodeString(addressHex)
			if err != nil {
				panic(err)
			}

			// The next 64 characters represent the amount, also padded to 32 bytes
			amountHex := data[64:]
			amountBytes, err := hex.DecodeString(amountHex)
			if err != nil {
				panic(err)
			}
			amountFunctionIdentifier := new(big.Int).SetBytes(amountBytes)

			// addressFunctionidentifier, err := hex.DecodeString(strings.TrimLeft(address, "0"))
			// if err != nil {
			// 	configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
			// }

			// amountFunctionIdentifier, err := strconv.ParseUint(amountHex, 16, 64)
			// if err != nil {
			// 	configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
			// }
			return []byte(tracerResponse.Result.Type), []byte(tracerResponse.Result.CallType), from, to, input, output, tracerResponse.Result.TraceAddress[:], value, gas, gasUsed, addressFunctionidentifier, amountFunctionIdentifier.Uint64()
		} else {
			return []byte(tracerResponse.Result.Type), []byte(tracerResponse.Result.CallType), from, to, input, output, tracerResponse.Result.TraceAddress[:], value, gas, gasUsed, nil, 0
		}
	}
	return nil, nil, nil, nil, 0, 0, nil, 0, 0, gasUsed, nil, 0
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
	var validators models.Validators

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getValidators",
		Params:  []interface{}{},
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

	err = json.Unmarshal([]byte(string(body)), &validators)

	return validators.ResultValidator
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
	// Assuming the contract address is one of the parameters
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
