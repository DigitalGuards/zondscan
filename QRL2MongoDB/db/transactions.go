package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

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
		// Only queue if this is actually a contract (new creation or interaction with existing contract)
		// This avoids queuing regular wallet addresses which would just be filtered out later
		if contractAddress != "" {
			// New contract creation - always queue
			QueuePotentialTokenContract(contractAddress, &tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp)
		} else if isContract && to != "" {
			// Transaction to an existing contract - queue for token processing
			QueuePotentialTokenContract(to, &tx, blockData.(models.ZondDatabaseBlock).Result.Timestamp)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
// This should be called after transaction processing is complete.
// Uses FindOneAndUpdate to atomically claim each work item, preventing duplicate
// processing if multiple goroutines call this function concurrently.
func ProcessTokenTransfersFromTransactions() {
	configs.Logger.Info("Processing of queued token contracts")

	collection := configs.GetCollection(configs.DB, "pending_token_contracts")

	// Count unprocessed items for logging
	countCtx, countCancel := context.WithTimeout(context.Background(), 30*time.Second)
	count, err := collection.CountDocuments(countCtx, bson.M{"processed": false})
	countCancel()
	if err != nil {
		configs.Logger.Error("Failed to count pending token contracts", zap.Error(err))
		return
	}

	configs.Logger.Info("Found pending token contracts to process", zap.Int64("count", count))
	if count == 0 {
		configs.Logger.Info("No pending token contracts to process")
		return
	}

	// Process each item by atomically claiming it with FindOneAndUpdate.
	// This prevents race conditions: only the goroutine that successfully flips
	// processed=false→true will execute processTokenContract for that item.
	processed := 0
	claimFilter := bson.M{"processed": false}
	claimUpdate := bson.M{"$set": bson.M{"processed": true}}
	findOneOpts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "contractAddress", Value: 1}, {Key: "txHash", Value: 1}}).
		SetReturnDocument(options.Before)

	for {
		var pending struct {
			ContractAddress string `bson:"contractAddress"`
			TxHash          string `bson:"txHash"`
			BlockNumber     string `bson:"blockNumber"`
			BlockTimestamp  string `bson:"blockTimestamp"`
		}

		claimCtx, claimCancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := collection.FindOneAndUpdate(claimCtx, claimFilter, claimUpdate, findOneOpts).Decode(&pending)
		claimCancel()

		if err == mongo.ErrNoDocuments {
			// No more unprocessed items
			break
		}
		if err != nil {
			configs.Logger.Error("Failed to claim pending token contract", zap.Error(err))
			break
		}

		configs.Logger.Debug("Processing token contract",
			zap.String("address", pending.ContractAddress),
			zap.String("txHash", pending.TxHash),
			zap.String("blockNumber", pending.BlockNumber))

		processTokenContract(pending.ContractAddress, pending.TxHash, pending.BlockNumber, pending.BlockTimestamp)
		processed++
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

		// Idempotent: the direct-calldata path uses logIndex="" (there's
		// no originating log) and tokenID="" (ERC-20 only). The unique
		// tuple for replay-detection is (txHash, contractAddress, "", "").
		exists, err := TokenTransferExists(txHash, targetAddress, "", "")
		if err == nil && exists {
			configs.Logger.Debug("Skipping duplicate direct token transfer",
				zap.String("txHash", txHash),
				zap.String("contract", targetAddress))
		} else {
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
		// Idempotent dedupe of the log-derived row. processTokenContract
		// runs only on ERC-20 contracts (the legacy ProcessTransferLogs
		// path), so tokenID is always "" here.
		exists, err := TokenTransferExists(txHash, targetAddress, transferEvent.LogIndex, "")
		if err != nil {
			configs.Logger.Error("Failed to check duplicate token transfer",
				zap.String("txHash", txHash),
				zap.String("contract", targetAddress),
				zap.String("logIndex", transferEvent.LogIndex),
				zap.Error(err))
			continue
		}
		if exists {
			configs.Logger.Debug("Skipping duplicate token transfer event",
				zap.String("txHash", txHash),
				zap.String("contract", targetAddress),
				zap.String("logIndex", transferEvent.LogIndex))
			continue
		}

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
			LogIndex:        transferEvent.LogIndex,
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

	// Convert value to float64 for display. Guard against empty/short
	// values, slicing [2:] on those panics; treat too-short as zero.
	value := new(big.Int)
	if len(tx.Value) > 2 {
		if _, ok := value.SetString(tx.Value[2:], 16); !ok {
			value.SetInt64(0)
		}
	}
	divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
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

			divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
			bigIntAsFloat := new(big.Float).SetInt(getBalanceResult)
			resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
			resultFloat64, _ := resultBigFloat.Float64()

			UpsertTransactions(address, resultFloat64, isContract)
		}
	}

	trace := rpc.CallDebugTraceTransaction(tx.Hash)
	if trace.Err != nil {
		configs.Logger.Warn("Debug trace failed; internal calls for this tx will be missing",
			zap.String("txHash", txHash), zap.Error(trace.Err))
	}
	// Persist the nested (depth >= 1) call frames: value moved by contract
	// code rather than by the outer transaction itself, e.g. an HTLC claim
	// paying out contract-held funds. The top-level frame is intentionally
	// not stored; it would only duplicate the transactionByAddress row.
	if err := StoreInternalCalls(trace.InternalCalls, txHash, blockTimestamp, blockNumber); err != nil {
		configs.Logger.Error("Failed to store internal calls",
			zap.String("txHash", txHash), zap.Error(err))
	}

	// Calculate fees using hex strings. Guard against empty/short
	// gasPrice, slicing [2:] on those panics; treat too-short as zero.
	gasPriceBig := new(big.Int)
	if len(gasPrice) > 2 {
		if _, ok := gasPriceBig.SetString(gasPrice[2:], 16); !ok {
			gasPriceBig.SetInt64(0)
		}
	}

	gasUsedBig := new(big.Int)
	// If trace.GasUsed is 0, try to use gasUsed from the transaction receipt
	if trace.GasUsed == 0 {
		// Get transaction receipt to obtain actual gas used
		receipt, err := rpc.GetTransactionReceipt(txHash)
		if err == nil && receipt != nil && receipt.Result.GasUsed != "" && len(receipt.Result.GasUsed) > 2 {
			gasUsedBig.SetString(receipt.Result.GasUsed[2:], 16)
			configs.Logger.Debug("Using gasUsed from receipt",
				zap.String("txHash", txHash),
				zap.String("gasUsed", receipt.Result.GasUsed))
		} else {
			// If receipt isn't available, use gas limit as a fallback
			// This is not accurate but better than 0
			if tx.Gas != "" && len(tx.Gas) > 2 {
				gasUsedBig.SetString(tx.Gas[2:], 16)
				configs.Logger.Debug("Using gas limit as fallback",
					zap.String("txHash", txHash),
					zap.String("gas", tx.Gas))
			} else {
				gasUsedBig.SetString(fmt.Sprintf("%x", trace.GasUsed), 16)
			}
		}
	} else {
		gasUsedBig.SetString(fmt.Sprintf("%x", trace.GasUsed), 16)
	}

	feesBig := new(big.Int).Mul(gasPriceBig, gasUsedBig)

	// Ensure fees are never zero for successful transactions. Apply the
	// minimal-fee floor to the wei integer (0.000001 QRL = 1e12 wei) so the
	// exact paidFeesWei string and the legacy paidFees float stay consistent.
	if feesBig.Sign() == 0 && statusTx == "0x1" {
		configs.Logger.Warn("Calculated fees is zero for a successful transaction, using minimal fee",
			zap.String("txHash", txHash))
		feesBig = big.NewInt(1_000_000_000_000)
	}

	// Legacy float fields, kept for backward-compatible numeric queries (e.g.
	// amount $gt 0). The exact, drift-free value travels alongside them as the
	// amountWei / paidFeesWei base-10 integer strings (value and feesBig).
	divisor = new(big.Float).SetFloat64(float64(utils.QUANTA))
	feesFloat := new(big.Float).SetInt(feesBig)
	feesResult := new(big.Float).Quo(feesFloat, divisor)
	fees, _ := feesResult.Float64()

	TransactionByAddressCollection(blockTimestamp, txType, from, to, txHash, valueFloat64, fees, blockNumber, value.String(), feesBig.String())
	TransferCollection(blockNumber, blockTimestamp, from, to, txHash, pk, signature, nonce, valueFloat64, data, contractAddress, statusTx, size, fees)
}

func TransferCollection(blockNumber string, blockTimestamp string, from string, to string, hash string, pk string, signature string, nonce string, value float64, data string, contractAddress string, status string, size string, paidFees float64) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	from = validation.ConvertToQAddress(from)
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}
	if contractAddress != "" {
		contractAddress = validation.ConvertToQAddress(contractAddress)
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.TransferCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

// StoreInternalCalls persists each flattened nested call frame of one
// transaction as an internalTransactionByAddress row. Shared by the live
// sync path (processTransactions) and the historical backfill command.
// A failed insert does not stop the remaining frames; all failures are
// joined into the returned error.
func StoreInternalCalls(calls []rpc.InternalCall, txHash string, blockTimestamp string, blockNumber string) error {
	var errs []error
	for _, call := range calls {
		_, err := InternalTransactionByAddressCollection(
			call.Type,
			strings.ToLower(call.Type),
			txHash,
			call.From,
			call.To,
			call.Input,
			call.Output,
			call.TraceAddress,
			call.Value,
			call.Gas,
			call.GasUsed,
			"",
			"0x0",
			blockTimestamp,
			blockNumber,
		)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func InternalTransactionByAddressCollection(transactionType string, callType string, hash string, from string, to string, input string, output string, traceAddress []int, value float64, gas string, gasUsed string, addressFunctionIdentifier string, amountFunctionIdentifier string, blockTimestamp string, blockNumber string) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	if from != "" {
		from = validation.ConvertToQAddress(from)
	}
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}
	if addressFunctionIdentifier != "" {
		addressFunctionIdentifier = validation.ConvertToQAddress(addressFunctionIdentifier)
	}

	// blockNumber (raw hex string) lets Rollback delete this row on a reorg via
	// the same $in companionFilter used for the other append-only collections.
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
		{Key: "blockNumber", Value: blockNumber},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.InternalTransactionByAddressCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the internalTransactionByAddress collection:", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// TransactionByAddressCollection persists a per-address transaction row.
// amount/paidFees are the legacy float64 columns (kept for backward-compatible
// numeric queries); amountWei/paidFeesWei are the exact base-10 wei integer
// strings the API uses to render QRL without float64 precision drift.
func TransactionByAddressCollection(timeStamp string, txType string, from string, to string, hash string, amount float64, paidFees float64, blockNumber string, amountWei string, paidFeesWei string) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	from = validation.ConvertToQAddress(from)
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}

	doc := bson.D{
		{Key: "txType", Value: txType},
		{Key: "from", Value: from},
		{Key: "to", Value: to},
		{Key: "txHash", Value: hash},
		{Key: "timeStamp", Value: timeStamp},
		{Key: "amount", Value: amount},
		{Key: "amountWei", Value: amountWei},
		{Key: "paidFees", Value: paidFees},
		{Key: "paidFeesWei", Value: paidFeesWei},
		{Key: "blockNumber", Value: blockNumber},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.TransactionByAddressCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

func UpsertTransactions(address string, value float64, isContract bool) (*mongo.UpdateResult, error) {
	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)
	filter := bson.D{{Key: "id", Value: address}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
		result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
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

	err := configs.AddressesCollections.FindOne(ctx, filter).Decode(&existingDoc)
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
		result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
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
	result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
	}
	return result, err
}

func GetContractByAddress(address string) *models.ContractInfo {
	address = validation.ConvertToQAddress(address)
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
}

// InitializePendingTokenContractsCollection ensures the pending token contracts collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist, avoiding destructive DropAll.
func InitializePendingTokenContractsCollection() error {
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing pending_token_contracts collection and indexes")

	// Create indexes for pending token contracts collection.
	// CreateMany is a no-op if the index already exists, so this is safe to call on restart.
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

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for pending token contracts",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized pending_token_contracts collection and indexes")
	return nil
}
