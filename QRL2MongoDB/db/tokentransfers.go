package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenTransfer stores a token transfer event in the database
func StoreTokenTransfer(transfer models.TokenTransfer) error {
	// Get explicit reference to the tokenTransfers collection
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Additional validation and normalization before inserting
	if transfer.From == "" {
		transfer.From = configs.QRLZeroAddress // Normalize empty from address to zero address
	}

	if transfer.To == "" {
		transfer.To = configs.QRLZeroAddress // Normalize empty to address to zero address
	}

	// Normalize addresses to canonical Q-prefix form
	transfer.From = validation.ConvertToQAddress(transfer.From)
	transfer.To = validation.ConvertToQAddress(transfer.To)
	transfer.ContractAddress = validation.ConvertToQAddress(transfer.ContractAddress)

	// Debug-level log for per-record operations; Info is reserved for batch summaries.
	configs.Logger.Debug("Inserting token transfer document",
		zap.String("token", transfer.TokenSymbol),
		zap.String("from", transfer.From),
		zap.String("to", transfer.To),
		zap.String("txHash", transfer.TxHash))

	_, err := collection.InsertOne(ctx, transfer)
	if err != nil {
		configs.Logger.Error("Failed to store token transfer",
			zap.String("txHash", transfer.TxHash),
			zap.String("token", transfer.TokenSymbol),
			zap.Error(err))
		return err
	}

	configs.Logger.Debug("Successfully stored token transfer in database",
		zap.String("token", transfer.TokenSymbol),
		zap.String("txHash", transfer.TxHash))
	return nil
}

// GetTokenTransfersByContract retrieves all transfers for a specific token contract
func GetTokenTransfersByContract(contractAddress string, skip, limit int64) ([]models.TokenTransfer, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := collection.Find(ctx,
		bson.M{"contractAddress": contractAddress},
		opts,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err = cursor.All(ctx, &transfers); err != nil {
		return nil, err
	}

	return transfers, nil
}

// GetTokenTransfersByAddress retrieves all transfers involving a specific address (as sender or receiver)
func GetTokenTransfersByAddress(address string, skip, limit int64) ([]models.TokenTransfer, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := collection.Find(ctx,
		bson.M{
			"$or": []bson.M{
				{"from": address},
				{"to": address},
			},
		},
		opts,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err = cursor.All(ctx, &transfers); err != nil {
		return nil, err
	}

	return transfers, nil
}

// TokenTransferExists checks if a specific token transfer is already
// persisted. The dedupe key is (txHash, contractAddress, logIndex):
//
//   - txHash + logIndex alone collide across token contracts when a
//     single tx routes through multiple tokens (a swap that transfers
//     tokenA at log 0 and tokenB at log 1 → both rows distinct on
//     logIndex, but the legacy direct-calldata path writes logIndex=""
//     for every contract it touches in the same tx, so contractAddress
//     is what keeps them apart).
//   - The legacy four-arg form (txHash + contractAddress + from + to)
//     collided on self-transfers (from == to) and on identical
//     bridge-style transfers within one tx.
func TokenTransferExists(txHash string, contractAddress string, logIndex string) (bool, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"txHash":          txHash,
		"contractAddress": contractAddress,
		"logIndex":        logIndex,
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to check if token transfer exists",
			zap.String("txHash", txHash),
			zap.String("contractAddress", contractAddress),
			zap.String("logIndex", logIndex),
			zap.Error(err))
		return false, err
	}

	return count > 0, nil
}

// ProcessBlockTokenTransfers processes all token transfers in a block
func ProcessBlockTokenTransfers(blockNumber string, blockTimestamp string) error {
	// Get logs for the Transfer event signature
	transferEventSignature := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	configs.Logger.Info("Searching for token transfers",
		zap.String("blockNumber", blockNumber),
		zap.String("eventSignature", transferEventSignature))

	response, err := rpc.ZondGetBlockLogs(blockNumber, []string{transferEventSignature})
	if err != nil {
		configs.Logger.Error("Failed to get logs for block",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return err
	}

	if response == nil || len(response.Result) == 0 {
		configs.Logger.Debug("No token transfer logs found in block",
			zap.String("blockNumber", blockNumber))
		return nil // No logs found
	}

	configs.Logger.Info("Found potential token transfer logs",
		zap.String("blockNumber", blockNumber),
		zap.Int("logCount", len(response.Result)))

	// Process each log
	tokenTransfersFound := 0
	for _, log := range response.Result {
		// Skip logs with insufficient topics
		if len(log.Topics) < 3 {
			configs.Logger.Debug("Skipping log with insufficient topics",
				zap.String("txHash", log.TransactionHash),
				zap.Int("topicCount", len(log.Topics)))
			continue
		}

		// Extract contract address
		contractAddress := log.Address
		configs.Logger.Debug("Processing potential token transfer",
			zap.String("contractAddress", contractAddress),
			zap.String("txHash", log.TransactionHash))

		// Use the consolidated token detection logic
		contract, isToken := EnsureTokenInDatabase(contractAddress, blockNumber, log.TransactionHash)
		if !isToken || contract == nil {
			configs.Logger.Debug("Contract is not a token, skipping",
				zap.String("address", contractAddress))
			continue
		}

		// Extract from and to addresses using canonical Q-prefix form.
		// topics[1] and topics[2] are 32-byte padded addresses; strip the 12-byte
		// zero-padding (24 hex chars) to recover the 20-byte address.
		from := "Q" + strings.ToLower(rpc.TrimLeftZeros(log.Topics[1][26:]))
		to := "Q" + strings.ToLower(rpc.TrimLeftZeros(log.Topics[2][26:]))

		configs.Logger.Debug("Token transfer details",
			zap.String("from", from),
			zap.String("to", to),
			zap.String("token", contract.Symbol))

		// Extract amount
		amount := log.Data

		// Check if this specific log already persisted (idempotent on
		// reprocess). Per-(tx, contract, logIndex) granularity matters:
		// a single tx may emit Transfer events from multiple token
		// contracts in any order.
		exists, err := TokenTransferExists(log.TransactionHash, contractAddress, log.LogIndex)
		if err != nil {
			configs.Logger.Error("Failed to check if token transfer exists",
				zap.String("txHash", log.TransactionHash),
				zap.Error(err))
			continue
		}

		if exists {
			// Skip duplicate transfers
			configs.Logger.Debug("Skipping duplicate token transfer",
				zap.String("txHash", log.TransactionHash))
			continue
		}

		// Normalize addresses to ensure consistency.
		if from == "" || from == "q" || from == "Q" {
			from = configs.QRLZeroAddress
		}

		if to == "" || to == "q" || to == "Q" {
			to = configs.QRLZeroAddress
		}

		// Debug-level log for per-record operations
		configs.Logger.Debug("Identified token transfer",
			zap.String("token", contract.Symbol),
			zap.String("from", from),
			zap.String("to", to),
			zap.String("amount", amount),
			zap.String("blockNumber", blockNumber))

		// Create token transfer record
		transfer := models.TokenTransfer{
			ContractAddress: contractAddress,
			From:            from,
			To:              to,
			Amount:          amount,
			BlockNumber:     blockNumber,
			TxHash:          log.TransactionHash,
			LogIndex:        log.LogIndex,
			Timestamp:       blockTimestamp,
			TokenSymbol:     contract.Symbol,
			TokenDecimals:   contract.Decimals,
			TokenName:       contract.Name,
			TransferType:    "event",
		}

		// Store the transfer
		err = StoreTokenTransfer(transfer)
		if err != nil {
			configs.Logger.Error("Failed to store token transfer",
				zap.String("txHash", log.TransactionHash),
				zap.Error(err))
			continue
		}
		tokenTransfersFound++

		// Update token balances
		if err = StoreTokenBalance(contractAddress, from, amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to update sender token balance",
				zap.String("address", from),
				zap.String("contractAddress", contractAddress),
				zap.Error(err))
		}

		if err = StoreTokenBalance(contractAddress, to, amount, blockNumber); err != nil {
			configs.Logger.Error("Failed to update recipient token balance",
				zap.String("address", to),
				zap.String("contractAddress", contractAddress),
				zap.Error(err))
		}
	}

	// Batch summary at Info level
	configs.Logger.Info("Finished processing token transfers",
		zap.String("blockNumber", blockNumber),
		zap.Int("transfersProcessed", tokenTransfersFound))

	return nil
}

// InitializeTokenTransfersCollection ensures the token transfers collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist — safe to call on every restart.
func InitializeTokenTransfersCollection() error {
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing tokenTransfers collection and indexes")

	// Migration: the previous schema enforced a UNIQUE index on `txHash`
	// alone, which silently dropped every Transfer event after the first
	// within any tx that emitted more than one (DEX swaps, batch mints,
	// fee-skim contracts). Drop it if it's still around — DropOne returns
	// IndexNotFound on fresh deployments and that's fine.
	if _, err := collection.Indexes().DropOne(ctx, "txHash_idx"); err != nil {
		msg := err.Error()
		// Acceptable: index never existed, or collection doesn't yet.
		if !strings.Contains(msg, "IndexNotFound") &&
			!strings.Contains(msg, "ns does not exist") &&
			!strings.Contains(msg, "NamespaceNotFound") {
			configs.Logger.Warn("Could not drop legacy unique txHash_idx — continuing; new index creation may also fail",
				zap.Error(err))
		}
	}

	// Create indexes for token transfers collection.
	// CreateMany does not drop existing indexes and is idempotent.
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("contract_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("from_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("to_block_idx"),
		},
		{
			// Non-unique. Provides a fast lookup path for
			// TokenTransferExists(txHash, contractAddress, logIndex)
			// and for any /tx/:hash → transfers query. Dedup is enforced
			// at the application layer via TokenTransferExists before
			// insert; a unique constraint here would re-introduce the
			// original bug for the legacy direct-calldata path (which
			// writes logIndex="").
			Keys: bson.D{
				{Key: "txHash", Value: 1},
				{Key: "contractAddress", Value: 1},
				{Key: "logIndex", Value: 1},
			},
			Options: options.Index().SetName("txHash_contract_logIndex_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for token transfers",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized tokenTransfers collection and indexes")
	return nil
}

// InitializeTokenBalancesCollection ensures the token balances collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist — safe to call on every restart.
func InitializeTokenBalancesCollection() error {
	collection := configs.GetTokenBalancesCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing tokenBalances collection and indexes")

	// Create indexes for token balances collection.
	// CreateMany does not drop existing indexes and is idempotent.
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "address", Value: 1},
			},
			Options: options.Index().SetName("contract_address_idx").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "address", Value: 1},
			},
			Options: options.Index().SetName("address_idx"),
		},
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
			},
			Options: options.Index().SetName("contract_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for token balances",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized tokenBalances collection and indexes")
	return nil
}
