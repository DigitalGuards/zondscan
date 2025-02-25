package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"fmt"
	"math/big"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func ProcessTransactions(blockData interface{}) {
	for _, tx := range blockData.(models.ZondDatabaseBlock).Result.Transactions {
		to, contractAddress, statusTx, isContract := processContracts(&tx)

		processTransactionData(&tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp, to, contractAddress, statusTx, isContract, blockData.(models.ZondDatabaseBlock).Result.Size)

		// Process token transfers for both contract creation and interaction
		targetAddress := to
		if contractAddress != "" {
			targetAddress = contractAddress
		}

		if targetAddress != "" {
			configs.Logger.Debug("Checking for token transfers",
				zap.String("targetAddress", targetAddress),
				zap.String("txHash", tx.Hash))

			// Check if this is a token contract
			contract := GetContractByAddress(targetAddress)
			if contract == nil {
				configs.Logger.Debug("Contract not found in database",
					zap.String("address", targetAddress))
			} else if !contract.IsToken {
				configs.Logger.Debug("Contract is not a token",
					zap.String("address", targetAddress))
			} else {
				configs.Logger.Debug("Found token contract",
					zap.String("address", targetAddress),
					zap.String("name", contract.Name),
					zap.String("symbol", contract.Symbol))

				// First check direct transfer calls
				from, recipient, amount := rpc.DecodeTransferEvent(tx.Data)
				if from != "" && recipient != "" && amount != "" {
					configs.Logger.Info("Found direct token transfer",
						zap.String("contract", targetAddress),
						zap.String("from", from),
						zap.String("to", recipient),
						zap.String("amount", amount))

					// Store token transfer
					transfer := models.TokenTransfer{
						ContractAddress: targetAddress,
						From:            from,
						To:              recipient,
						Amount:          amount,
						BlockNumber:     tx.BlockNumber,
						TxHash:          tx.Hash,
						Timestamp:       blockData.(models.ZondDatabaseBlock).Result.Timestamp,
						TokenSymbol:     contract.Symbol,
						TokenDecimals:   contract.Decimals,
						TokenName:       contract.Name,
						TransferType:    "direct",
					}
					if err := StoreTokenTransfer(transfer); err != nil {
						configs.Logger.Error("Failed to store token transfer",
							zap.String("txHash", tx.Hash),
							zap.Error(err))
					}

					// Update token balances
					if err := StoreTokenBalance(targetAddress, from, amount, tx.BlockNumber); err != nil {
						configs.Logger.Error("Failed to store token balance for sender",
							zap.String("contract", targetAddress),
							zap.String("holder", from),
							zap.Error(err))
					}
					if err := StoreTokenBalance(targetAddress, recipient, amount, tx.BlockNumber); err != nil {
						configs.Logger.Error("Failed to store token balance for recipient",
							zap.String("contract", targetAddress),
							zap.String("holder", recipient),
							zap.Error(err))
					}
				}

				// Then check transfer events in logs
				receipt, err := rpc.GetTransactionReceipt(tx.Hash)
				if err != nil {
					configs.Logger.Error("Failed to get transaction receipt",
						zap.String("hash", tx.Hash),
						zap.Error(err))
				} else {
					transfers := rpc.ProcessTransferLogs(receipt)
					for _, transferEvent := range transfers {
						configs.Logger.Info("Found token transfer event",
							zap.String("contract", targetAddress),
							zap.String("from", transferEvent.From),
							zap.String("to", transferEvent.To),
							zap.String("amount", transferEvent.Amount))

						// Store token transfer
						transfer := models.TokenTransfer{
							ContractAddress: targetAddress,
							From:            transferEvent.From,
							To:              transferEvent.To,
							Amount:          transferEvent.Amount,
							BlockNumber:     tx.BlockNumber,
							TxHash:          tx.Hash,
							Timestamp:       blockData.(models.ZondDatabaseBlock).Result.Timestamp,
							TokenSymbol:     contract.Symbol,
							TokenDecimals:   contract.Decimals,
							TokenName:       contract.Name,
							TransferType:    "event",
						}
						if err := StoreTokenTransfer(transfer); err != nil {
							configs.Logger.Error("Failed to store token transfer",
								zap.String("txHash", tx.Hash),
								zap.Error(err))
						}

						// Update token balances
						if err := StoreTokenBalance(targetAddress, transferEvent.From, transferEvent.Amount, tx.BlockNumber); err != nil {
							configs.Logger.Error("Failed to store token balance for sender",
								zap.String("contract", targetAddress),
								zap.String("holder", transferEvent.From),
								zap.Error(err))
						}
						if err := StoreTokenBalance(targetAddress, transferEvent.To, transferEvent.Amount, tx.BlockNumber); err != nil {
							configs.Logger.Error("Failed to store token balance for recipient",
								zap.String("contract", targetAddress),
								zap.String("holder", transferEvent.To),
								zap.Error(err))
						}
					}
				}
			}
		}
	}
}

func processTransactionData(tx *models.Transaction, blockTimestamp string, to string, contractAddress string, statusTx string, isContract bool, size string) {
	from := tx.From
	txHash := tx.Hash
	blockNumber := tx.BlockNumber
	gasPrice := tx.GasPrice
	pk := tx.PublicKey
	signature := tx.Signature
	data := tx.Data
	nonce := tx.Nonce
	txType := tx.Type

	// Convert value to float64 for display
	value := new(big.Int)
	value.SetString(tx.Value[2:], 16)
	divisor := new(big.Float).SetFloat64(float64(configs.QUANTA))
	bigIntAsFloat := new(big.Float).SetInt(value)
	resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
	valueFloat64, _ := resultBigFloat.Float64()

	hashmap := map[string]string{"from": tx.From, "to": tx.To}

	for _, address := range hashmap {
		if address != "" {
			responseBalance, err := rpc.GetBalance(address)
			if err != nil {
				configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
				continue
			}

			getBalanceResult := new(big.Int)
			if responseBalance != "" && len(responseBalance) > 2 {
				getBalanceResult.SetString(responseBalance[2:], 16)
			} else {
				configs.Logger.Warn("Invalid balance response", zap.String("balance", responseBalance))
				continue
			}

			divisor := new(big.Float).SetFloat64(float64(configs.QUANTA))
			bigIntAsFloat := new(big.Float).SetInt(getBalanceResult)
			resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
			resultFloat64, _ := resultBigFloat.Float64()

			UpsertTransactions(address, resultFloat64, isContract)
		}
	}

	transactionType, callType, fromInternal, toInternal, inputInternal, outputInternal, InternalTracerAddress, valueInternal, gasInternal, gasUsedInternal, addressFunctionIdentifier, amountFunctionIdentifier := rpc.CallDebugTraceTransaction(tx.Hash)
	if transactionType == "CALL" || InternalTracerAddress != nil {
		InternalTransactionByAddressCollection(transactionType, callType, txHash, fromInternal, toInternal, fmt.Sprintf("0x%x", inputInternal), fmt.Sprintf("0x%x", outputInternal), InternalTracerAddress, float64(valueInternal), fmt.Sprintf("0x%x", gasInternal), fmt.Sprintf("0x%x", gasUsedInternal), addressFunctionIdentifier, fmt.Sprintf("0x%x", amountFunctionIdentifier), blockTimestamp)
	}

	// Calculate fees using hex strings
	gasPriceBig := new(big.Int)
	gasPriceBig.SetString(gasPrice[2:], 16)
	gasUsedBig := new(big.Int)
	gasUsedBig.SetString(fmt.Sprintf("%x", gasUsedInternal), 16)
	feesBig := new(big.Int).Mul(gasPriceBig, gasUsedBig)

	divisor = new(big.Float).SetFloat64(float64(configs.QUANTA))
	feesFloat := new(big.Float).SetInt(feesBig)
	feesResult := new(big.Float).Quo(feesFloat, divisor)
	fees, _ := feesResult.Float64()

	TransactionByAddressCollection(blockTimestamp, txType, from, to, txHash, valueFloat64, fees, blockNumber)
	TransferCollection(blockNumber, blockTimestamp, from, to, txHash, pk, signature, nonce, valueFloat64, data, contractAddress, statusTx, size, fees)
}

func TransferCollection(blockNumber string, blockTimestamp string, from string, to string, hash string, pk string, signature string, nonce string, value float64, data string, contractAddress string, status string, size string, paidFees float64) (*mongo.InsertOneResult, error) {
	var doc bson.D

	baseDoc := bson.D{
		{Key: "blockNumber", Value: blockNumber},
		{Key: "blockTimestamp", Value: blockTimestamp},
		{Key: "from", Value: from},
		{Key: "txHash", Value: hash},
		{Key: "pk", Value: pk},
		{Key: "signature", Value: signature},
		{Key: "nonce", Value: nonce},
		{Key: "value", Value: value},
		{Key: "status", Value: status},
		{Key: "size", Value: size},
		{Key: "paidFees", Value: paidFees},
	}

	if contractAddress == "" {
		doc = append(baseDoc, bson.E{Key: "to", Value: to})
		if data != "" {
			doc = append(doc, bson.E{Key: "data", Value: data})
		}
	} else {
		doc = append(baseDoc, bson.E{Key: "contractAddress", Value: contractAddress})
		if data != "" {
			doc = append(doc, bson.E{Key: "data", Value: data})
		}
	}

	result, err := configs.TransferCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

func InternalTransactionByAddressCollection(transactionType string, callType string, hash string, from string, to string, input string, output string, traceAddress []int, value float64, gas string, gasUsed string, addressFunctionIdentifier string, amountFunctionIdentifier string, blockTimestamp string) (*mongo.InsertOneResult, error) {
	doc := bson.D{
		{Key: "type", Value: transactionType},
		{Key: "callType", Value: callType},
		{Key: "hash", Value: hash},
		{Key: "from", Value: from},
		{Key: "to", Value: to},
		{Key: "input", Value: input},
		{Key: "output", Value: output},
		{Key: "traceAddress", Value: traceAddress},
		{Key: "value", Value: value},
		{Key: "gas", Value: gas},
		{Key: "gasUsed", Value: gasUsed},
		{Key: "addressFunctionIdentifier", Value: addressFunctionIdentifier},
		{Key: "amountFunctionIdentifier", Value: amountFunctionIdentifier},
		{Key: "blockTimestamp", Value: blockTimestamp},
	}

	result, err := configs.InternalTransactionByAddressCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the internalTransactionByAddress collection:", zap.Error(err))
		return nil, err
	}

	return result, nil
}

func TransactionByAddressCollection(timeStamp string, txType string, from string, to string, hash string, amount float64, paidFees float64, blockNumber string) (*mongo.InsertOneResult, error) {
	doc := bson.D{
		{Key: "txType", Value: txType},
		{Key: "from", Value: from},
		{Key: "to", Value: to},
		{Key: "txHash", Value: hash},
		{Key: "timeStamp", Value: timeStamp},
		{Key: "amount", Value: amount},
		{Key: "paidFees", Value: paidFees},
		{Key: "blockNumber", Value: blockNumber},
	}

	result, err := configs.TransactionByAddressCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

func UpsertTransactions(address string, value float64, isContract bool) (*mongo.UpdateResult, error) {
	filter := bson.D{{Key: "id", Value: address}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "id", Value: address},
			{Key: "balance", Value: value},
			{Key: "isContract", Value: isContract},
		}},
	}
	opts := options.Update().SetUpsert(true)
	result, err := configs.AddressesCollections.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
	}
	return result, err
}

func GetContractByAddress(address string) *models.ContractInfo {
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo
	err := collection.FindOne(context.Background(), bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
}
