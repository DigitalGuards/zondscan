package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"QRLtoMongoDB-PoS/rpc"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func ProcessTransactions(blockData interface{}) {
	for _, tx := range blockData.(models.ZondDatabaseBlock).Result.Transactions {
		to, contractAddressByte, statusTx, isContract := processContracts(&tx)

		processXMSSBitfield(tx.From[0:3], tx.Signature[2:10])
		processTransactionData(&tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp, to, contractAddressByte, statusTx, isContract, blockData.(models.ZondDatabaseBlock).Result.Size)
	}
}

func processTransactionData(tx *models.Transaction, blockTimestamp uint64, to []byte, contractAddressByte []byte, statusTx uint8, isContract bool, size uint64) {
	from, err := hex.DecodeString(tx.From[2:])
	if err != nil {
		configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
	}

	txHash, err := hex.DecodeString(tx.Hash[2:])
	if err != nil {
		configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
	}

	blockNumber, err := strconv.ParseUint(tx.BlockNumber[2:], 16, 64)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	gasPrice, err := strconv.ParseUint(tx.GasPrice[2:], 16, 64)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	pk, err := hex.DecodeString(tx.PublicKey[2:])
	if err != nil {
		configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
	}

	signature, err := hex.DecodeString(tx.Signature[2:])
	if err != nil {
		configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
	}

	var data []byte

	data = nil

	if tx.Data != "" {
		data, err = hex.DecodeString(tx.Data[2:])
		if err != nil {
			configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
		}
	}

	nonce, err := strconv.ParseUint(tx.Nonce[2:], 16, 64)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	value := new(big.Int)
	value.SetString(tx.Value[2:], 16)

	divisor := new(big.Float).SetFloat64(float64(configs.QUANTA))

	bigIntAsFloat := new(big.Float).SetInt(value)

	resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)

	valueFloat32, _ := resultBigFloat.Float32()

	txType, err := strconv.ParseUint(tx.Type[2:], 16, 8)
	if err != nil {
		configs.Logger.Warn("Failed to ParseUint: ", zap.Error(err))
	}

	hashmap := map[string]string{"from": tx.From, "to": tx.To}

	for _, address := range hashmap {
		if address != "" {
			responseBalance, err := rpc.GetBalance(address)
			if err != nil {
				configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
				continue
			}

			var Balance models.GetBalance
			err = json.Unmarshal([]byte(responseBalance), &Balance)
			if err != nil {
				configs.Logger.Warn("Failed to parse JSON response: ", zap.Error(err))
				continue
			}

			addressBytes, err := hex.DecodeString(address[2:])
			if err != nil {
				configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
				continue
			}

			getBalanceResult := new(big.Int)
			getBalanceResult.SetString(Balance.Result[2:], 16)

			divisor := new(big.Float).SetFloat64(float64(configs.QUANTA))

			bigIntAsFloat := new(big.Float).SetInt(getBalanceResult)

			resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)

			resultFloat32, _ := resultBigFloat.Float32()

			UpsertTransactions(addressBytes, resultFloat32, isContract)
		}
	}

	transactionType, callType, fromInternal, toInternal, inputInternal, outputInternal, InternalTracerAddress, valueInternal, gasInternal, gasUsedInternal, addressFunctionIdentifier, amountFunctionIdentifier := rpc.CallDebugTraceTransaction(tx.Hash)
	if string(transactionType[:]) == "CALL" || InternalTracerAddress != nil {
		InternalTransactionByAddressCollection(transactionType, callType, txHash, fromInternal, toInternal, inputInternal, outputInternal, InternalTracerAddress, float32(valueInternal), gasInternal, gasUsedInternal, addressFunctionIdentifier, amountFunctionIdentifier, blockTimestamp)
	}
	TransactionByAddressCollection(blockTimestamp, uint8(txType), from, to, txHash, valueFloat32, (float32(gasPrice*gasUsedInternal) / configs.QUANTA))
	TransferCollection(blockNumber, blockTimestamp, from, to, txHash, pk, signature, nonce, valueFloat32, data, contractAddressByte, uint8(statusTx), size, (float32(gasPrice*gasUsedInternal) / configs.QUANTA))
}

func TransferCollection(blockNumber uint64, blockTimestamp uint64, from []byte, to []byte, hash []byte, pk []byte, signature []byte, nonce uint64, value float32, data []byte, contractAddress []byte, status uint8, size uint64, paidFees float32) (*mongo.InsertOneResult, error) {
	var doc primitive.D

	if contractAddress == nil {
		doc = bson.D{{"blockNumber", blockNumber}, {"blockTimestamp", blockTimestamp}, {"from", from}, {"to", to}, {"txHash", hash}, {"pk", pk}, {"signature", signature}, {"nonce", nonce}, {"value", value}, {"status", status}, {"size", size}, {"paidFees", paidFees}}

		if data != nil {
			doc = bson.D{{"blockNumber", blockNumber}, {"blockTimestamp", blockTimestamp}, {"from", from}, {"to", to}, {"txHash", hash}, {"pk", pk}, {"signature", signature}, {"nonce", nonce}, {"value", value}, {"data", data}, {"status", status}, {"size", size}, {"paidFees", paidFees}}
		}
	} else {
		doc = bson.D{{"blockNumber", blockNumber}, {"blockTimestamp", blockTimestamp}, {"from", from}, {"txHash", hash}, {"pk", pk}, {"signature", signature}, {"nonce", nonce}, {"value", value}, {"contractAddress", contractAddress}, {"status", status}, {"size", size}, {"paidFees", paidFees}}

		if data != nil {
			doc = bson.D{{"blockNumber", blockNumber}, {"blockTimestamp", blockTimestamp}, {"from", from}, {"txHash", hash}, {"pk", pk}, {"signature", signature}, {"nonce", nonce}, {"value", value}, {"contractAddress", contractAddress}, {"data", data}, {"status", status}, {"size", size}, {"paidFees", paidFees}}
		}
	}

	result, err := configs.TransferCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

func InternalTransactionByAddressCollection(transactionType []byte, callType []byte, hash []byte, from []byte, to []byte, input uint64, output uint64, traceAddress []int, value float32, gas uint64, gasUsed uint64, addressFunctionIdentifier []byte, amountFunctionIdentifier uint64, blockTimestamp uint64) (*mongo.InsertOneResult, error) {
	doc := bson.D{
		{"type", transactionType},
		{"callType", callType},
		{"hash", hash},
		{"from", from},
		{"to", to},
		{"input", input},
		{"output", output},
		{"traceAddress", traceAddress},
		{"value", value},
		{"gas", gas},
		{"gasUsed", gasUsed},
		{"addressFunctionIdentifier", addressFunctionIdentifier},
		{"amountFunctionIdentifier", amountFunctionIdentifier},
		{"blockTimestamp", blockTimestamp},
	}

	result, err := configs.InternalTransactionByAddressCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the internalTransactionByAddress collection:", zap.Error(err))
		return nil, err
	}

	fmt.Println(result)

	return result, nil
}

func TransactionByAddressCollection(timeStamp uint64, txType uint8, from []byte, to []byte, hash []byte, amount float32, paidFees float32) (*mongo.InsertOneResult, error) {
	var doc primitive.D
	var result *mongo.InsertOneResult
	var err error

	doc = bson.D{{"txType", txType}, {"from", from}, {"to", to}, {"txHash", hash}, {"timeStamp", timeStamp}, {"amount", amount}, {"paidFees", paidFees}}
	result, err = configs.TransactionByAddressCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

func UpsertTransactions(address []byte, value float32, isContract bool) (*mongo.UpdateResult, error) {
	filter := bson.D{{"id", address}}
	update := bson.D{
		{
			Key: "$set",
			Value: bson.D{
				{Key: "id", Value: address},
			},
		},
		{
			Key: "$set",
			Value: bson.D{
				{Key: "balance", Value: value},
			},
		},
		{
			Key: "$set",
			Value: bson.D{
				{Key: "isContract", Value: isContract},
			},
		},
	}
	opts := options.Update().SetUpsert(true)
	result, err := configs.AddressesCollections.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
	}
	return result, err
}
