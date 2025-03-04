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

// ProcessTransactions processes only transaction data without token logic
func ProcessTransactions(blockData interface{}) {
	for _, tx := range blockData.(models.ZondDatabaseBlock).Result.Transactions {
		to, contractAddress, statusTx, isContract := processContracts(&tx)

		processTransactionData(&tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp, to, contractAddress, statusTx, isContract, blockData.(models.ZondDatabaseBlock).Result.Size)

		// Store contract addresses for later token processing
		// instead of processing them inline
		if contractAddress != "" || to != "" {
			targetAddress := to
			if contractAddress != "" {
				targetAddress = contractAddress
			}

			// Only queue it if it's a non-empty address
			if targetAddress != "" {
				QueuePotentialTokenContract(targetAddress, &tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp)
			}
		}
	}
}

// QueuePotentialTokenContract stores a mapping of potential token contract addresses
// to be processed later in a batch
func QueuePotentialTokenContract(address string, tx *models.Transaction, blockTimestamp string) {
	// Skip if the address is empty
	if address == "" {
		return
	}

	// Use the pending contracts collection to store addresses
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	if collection == nil {
		configs.Logger.Error("Failed to get pending_token_contracts collection")
		return
	}

	ctx := context.Background()

	// Create the document to insert
	doc := bson.M{
		"contractAddress": address,
		"txHash":          tx.Hash,
		"blockNumber":     tx.BlockNumber,
		"blockTimestamp":  blockTimestamp,
		"processed":       false,
	}

	// Use upsert to prevent duplicates
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"contractAddress": address,
		"txHash":          tx.Hash,
	}

	_, err := collection.UpdateOne(ctx, filter, bson.M{"$set": doc}, opts)
	if err != nil {
		configs.Logger.Error("Failed to queue potential token contract",
			zap.String("address", address),
			zap.String("txHash", tx.Hash),
			zap.Error(err))
	} else {
		configs.Logger.Debug("Queued potential token contract for later processing",
			zap.String("address", address),
			zap.String("txHash", tx.Hash),
			zap.String("blockNumber", tx.BlockNumber))
	}
}

// ProcessTokenTransfersFromTransactions processes token transfers for queued contracts
// This should be called after transaction processing is complete
func ProcessTokenTransfersFromTransactions() {
	configs.Logger.Info("Processing of queued token contracts")

	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	ctx := context.Background()

	// Find unprocessed contract addresses
	filter := bson.M{"processed": false}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to count pending token contracts", zap.Error(err))
		return
	}

	configs.Logger.Info("Found pending token contracts to process", zap.Int64("count", count))

	if count == 0 {
		configs.Logger.Info("No pending token contracts to process")
		return
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to query pending token contracts", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	// Process each pending contract
	processed := 0
	for cursor.Next(ctx) {
		var pending struct {
			ContractAddress string `bson:"contractAddress"`
			TxHash          string `bson:"txHash"`
			BlockNumber     string `bson:"blockNumber"`
			BlockTimestamp  string `bson:"blockTimestamp"`
		}

		if err := cursor.Decode(&pending); err != nil {
			configs.Logger.Error("Failed to decode pending token contract", zap.Error(err))
			continue
		}

		// Process the token contract
		configs.Logger.Info("Processing token contract",
			zap.String("address", pending.ContractAddress),
			zap.String("txHash", pending.TxHash),
			zap.String("blockNumber", pending.BlockNumber))

		processTokenContract(pending.ContractAddress, pending.TxHash, pending.BlockNumber, pending.BlockTimestamp)
		processed++

		// Mark as processed
		updateFilter := bson.M{
			"contractAddress": pending.ContractAddress,
			"txHash":          pending.TxHash,
		}
		_, err := collection.UpdateOne(ctx, updateFilter, bson.M{"$set": bson.M{"processed": true}})
		if err != nil {
			configs.Logger.Error("Failed to mark token contract as processed",
				zap.String("address", pending.ContractAddress),
				zap.Error(err))
		}
	}

	configs.Logger.Info("Completed batch processing of token contracts", zap.Int("processed", processed))
}

// processTokenContract processes a single token contract address
func processTokenContract(targetAddress string, txHash string, blockNumber string, blockTimestamp string) {
	configs.Logger.Debug("Checking for token transfers",
		zap.String("targetAddress", targetAddress),
		zap.String("txHash", txHash))

	// Check if this is a token contract
	contract := GetContractByAddress(targetAddress)
	if contract == nil {
		configs.Logger.Debug("Contract not found in database",
			zap.String("address", targetAddress))
		return
	}

	if !contract.IsToken {
		configs.Logger.Debug("Contract is not a token",
			zap.String("address", targetAddress))
		return
	}

	configs.Logger.Debug("Found token contract",
		zap.String("address", targetAddress),
		zap.String("name", contract.Name),
		zap.String("symbol", contract.Symbol))

	// Get transaction details
	txDetails, err := rpc.GetTxDetailsByHash(txHash)
	if err != nil {
		configs.Logger.Error("Failed to get transaction details",
			zap.String("txHash", txHash),
			zap.Error(err))
		return
	}

	// First check direct transfer calls
	from, recipient, amount := rpc.DecodeTransferEvent(txDetails.Input)
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
			BlockNumber:     blockNumber,
			TxHash:          txHash,
			Timestamp:       blockTimestamp,
			TokenSymbol:     contract.Symbol,
			TokenDecimals:   contract.Decimals,
			TokenName:       contract.Name,
			TransferType:    "direct",
		}
		if err := StoreTokenTransfer(transfer); err != nil {
			configs.Logger.Error("Failed to store token transfer",
				zap.String("txHash", txHash),
				zap.Error(err))
		}

		// Update token balances
		if err := StoreTokenBalance(targetAddress, from, amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to store token balance for sender",
				zap.String("contract", targetAddress),
				zap.String("holder", from),
				zap.Error(err))
		}
		if err := StoreTokenBalance(targetAddress, recipient, amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to store token balance for recipient",
				zap.String("contract", targetAddress),
				zap.String("holder", recipient),
				zap.Error(err))
		}
	}

	// Then check transfer events in logs
	receipt, err := rpc.GetTransactionReceipt(txHash)
	if err != nil {
		configs.Logger.Error("Failed to get transaction receipt",
			zap.String("hash", txHash),
			zap.Error(err))
		return
	}

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
			BlockNumber:     blockNumber,
			TxHash:          txHash,
			Timestamp:       blockTimestamp,
			TokenSymbol:     contract.Symbol,
			TokenDecimals:   contract.Decimals,
			TokenName:       contract.Name,
			TransferType:    "event",
		}
		if err := StoreTokenTransfer(transfer); err != nil {
			configs.Logger.Error("Failed to store token transfer",
				zap.String("txHash", txHash),
				zap.Error(err))
		}

		// Update token balances
		if err := StoreTokenBalance(targetAddress, transferEvent.From, transferEvent.Amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to store token balance for sender",
				zap.String("contract", targetAddress),
				zap.String("holder", transferEvent.From),
				zap.Error(err))
		}
		if err := StoreTokenBalance(targetAddress, transferEvent.To, transferEvent.Amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to store token balance for recipient",
				zap.String("contract", targetAddress),
				zap.String("holder", transferEvent.To),
				zap.Error(err))
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

	// If this is flagged as a contract, update with that information
	if isContract {
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "id", Value: address},
				{Key: "balance", Value: value},
				{Key: "isContract", Value: true}, // Always set to true if we know it's a contract
			}},
		}
		opts := options.Update().SetUpsert(true)
		result, err := configs.AddressesCollections.UpdateOne(context.TODO(), filter, update, opts)
		if err != nil {
			configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
		}
		return result, err
	}

	// If not flagged as a contract, we need to check if it's already marked as a contract
	// to avoid overwriting that information
	var existingDoc struct {
		IsContract bool `bson:"isContract"`
	}

	err := configs.AddressesCollections.FindOne(context.TODO(), filter).Decode(&existingDoc)
	if err == nil && existingDoc.IsContract {
		// It's already marked as a contract, so keep that information
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "id", Value: address},
				{Key: "balance", Value: value},
				// Don't update isContract field since we want to keep it as true
			}},
		}
		opts := options.Update().SetUpsert(true)
		result, err := configs.AddressesCollections.UpdateOne(context.TODO(), filter, update, opts)
		if err != nil {
			configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
		}
		return result, err
	}

	// If it's not in our database or not marked as a contract, proceed with the regular update
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

// InitializePendingTokenContractsCollection ensures the pending token contracts collection is set up with proper indexes
func InitializePendingTokenContractsCollection() error {
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	ctx := context.Background()

	configs.Logger.Info("Initializing pending_token_contracts collection and indexes")

	// Create indexes for pending token contracts collection
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "txHash", Value: 1},
			},
			Options: options.Index().SetName("contract_tx_idx").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "processed", Value: 1},
			},
			Options: options.Index().SetName("processed_idx"),
		},
	}

	// First drop any existing indexes to avoid conflicts
	_, err := collection.Indexes().DropAll(ctx)
	if err != nil {
		configs.Logger.Warn("Failed to drop existing indexes, attempting to continue",
			zap.Error(err))
	}

	// Create the new indexes
	_, err = collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for pending token contracts",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized pending_token_contracts collection and indexes")
	return nil
}
