package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// TokenSyncConfig holds configuration for token sync operations
type TokenSyncConfig struct {
	BatchSize      int
	BatchDelayMs   int
	QueryTimeoutSec int
}

// DefaultTokenSyncConfig returns the default token sync configuration
func DefaultTokenSyncConfig() TokenSyncConfig {
	return TokenSyncConfig{
		BatchSize:       10,
		BatchDelayMs:    86,
		QueryTimeoutSec: 30,
	}
}

// ProcessTokensAfterInitialSync handles token transfer processing after the initial block sync is complete.
// It queries blocks with transactions and processes token transfers in batches.
func ProcessTokensAfterInitialSync(initialSyncStart string, maxHex string) {
	config := DefaultTokenSyncConfig()
	processTokensWithConfig(initialSyncStart, maxHex, config)
}

// processTokensWithConfig processes tokens with the given configuration
func processTokensWithConfig(initialSyncStart string, maxHex string, config TokenSyncConfig) {
	configs.Logger.Info("Beginning post-sync token transfer processing",
		zap.String("from_block", initialSyncStart),
		zap.String("to_block", maxHex))

	// Get blocks with transactions only
	blocksWithTxs, err := getBlocksWithTransactions(initialSyncStart, maxHex, config.QueryTimeoutSec)
	if err != nil {
		configs.Logger.Error("Failed to query blocks with transactions", zap.Error(err))
		return
	}

	if len(blocksWithTxs) == 0 {
		configs.Logger.Info("No blocks with transactions found in range")
		return
	}

	configs.Logger.Info("Found blocks with transactions to process",
		zap.Int("count", len(blocksWithTxs)))

	// Process token transfers in batches
	processTokenTransferBatches(blocksWithTxs, config)
}

// getBlocksWithTransactions queries the database for blocks that have at least one transaction
func getBlocksWithTransactions(fromBlock, toBlock string, timeoutSec int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	filter := bson.M{
		"result.number": bson.M{
			"$gte": fromBlock,
			"$lte": toBlock,
		},
		"result.transactions.0": bson.M{"$exists": true},
	}

	projection := bson.M{"result.number": 1, "_id": 0}

	cursor, err := configs.BlocksCollections.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var blocksWithTxs []string
	for cursor.Next(ctx) {
		var block struct {
			Result struct {
				Number string `bson:"number"`
			} `bson:"result"`
		}

		if err := cursor.Decode(&block); err != nil {
			configs.Logger.Error("Failed to decode block", zap.Error(err))
			continue
		}

		blocksWithTxs = append(blocksWithTxs, block.Result.Number)
	}

	return blocksWithTxs, nil
}

// processTokenTransferBatches processes token transfers in configurable batches
func processTokenTransferBatches(blocksWithTxs []string, config TokenSyncConfig) {
	totalProcessed := 0
	batchCounter := 0

	for i := 0; i < len(blocksWithTxs); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(blocksWithTxs) {
			end = len(blocksWithTxs)
		}

		batchBlocks := blocksWithTxs[i:end]
		batchSize := len(batchBlocks)

		configs.Logger.Info("Processing token transfers batch",
			zap.Int("batch", batchCounter),
			zap.Int("size", batchSize))

		for _, blockNumber := range batchBlocks {
			ProcessTokenTransfersForBlock(blockNumber)
			totalProcessed++
		}

		configs.Logger.Info("Completed token transfer batch",
			zap.Int("batch", batchCounter),
			zap.Int("blocks_processed", batchSize),
			zap.Int("total_processed", totalProcessed))

		batchCounter++

		// Add delay between batches to prevent overwhelming the node
		if config.BatchDelayMs > 0 {
			time.Sleep(time.Duration(config.BatchDelayMs) * time.Millisecond)
		}
	}

	configs.Logger.Info("Completed token transfer processing for all blocks with transactions",
		zap.Int("total_blocks_processed", totalProcessed))
}

// ProcessTokenTransfersForBlock processes token transfers in a single block
func ProcessTokenTransfersForBlock(blockNumber string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	configs.Logger.Info("Starting token transfer processing for block",
		zap.String("blockNumber", blockNumber))

	// Get block from database to get timestamp
	filter := bson.M{"result.number": blockNumber}
	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter).Decode(&block)
	if err != nil {
		configs.Logger.Error("Failed to get block for token transfer processing",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return
	}

	configs.Logger.Info("Retrieved block for token transfer processing",
		zap.String("blockNumber", blockNumber),
		zap.String("blockHash", block.Result.Hash),
		zap.String("timestamp", block.Result.Timestamp))

	// Skip if block has no transactions
	if len(block.Result.Transactions) == 0 {
		configs.Logger.Debug("Skipping token transfer processing for empty block",
			zap.String("blockNumber", blockNumber))
		return
	}

	// Process token transfers
	configs.Logger.Info("Calling ProcessBlockTokenTransfers",
		zap.String("blockNumber", blockNumber))

	err = db.ProcessBlockTokenTransfers(blockNumber, block.Result.Timestamp)
	if err != nil {
		configs.Logger.Error("Failed to process token transfers for block",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
	} else {
		configs.Logger.Info("Processed token transfers for block",
			zap.String("blockNumber", blockNumber))
	}
}

// InitializeTokenCollections initializes the token-related MongoDB collections.
// It attempts all initializations even if individual ones fail, collecting errors.
func InitializeTokenCollections() error {
	configs.Logger.Info("Initializing token collections")

	var initErrors []error

	// Initialize token transfers collection
	if err := db.InitializeTokenTransfersCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token transfers collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token transfers collection")
	}

	// Initialize token balances collection
	if err := db.InitializeTokenBalancesCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token balances collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token balances collection")
	}

	// Return first error if any occurred (maintaining original behavior of returning an error)
	if len(initErrors) > 0 {
		configs.Logger.Error("Token collection initialization completed with errors",
			zap.Int("error_count", len(initErrors)))
		return initErrors[0]
	}

	return nil
}

// GetTokenSyncRange calculates the range of blocks to process for token sync
func GetTokenSyncRange(lastSyncedBlock string, maxHex string) (string, string) {
	initialSyncStart := db.GetLastKnownBlockNumberFromInitialSync()
	if initialSyncStart == "0x0" {
		initialSyncStart = "0x1"
	}

	// Verify we aren't trying to process tokens beyond what's actually synced
	if utils.CompareHexNumbers(lastSyncedBlock, "0x0") > 0 &&
		utils.CompareHexNumbers(maxHex, lastSyncedBlock) > 0 {
		maxHex = lastSyncedBlock
		configs.Logger.Info("Limiting token processing to last synced block",
			zap.String("lastSyncedBlock", lastSyncedBlock))
	}

	return initialSyncStart, maxHex
}

// StoreInitialSyncStartBlock stores the starting block number for the initial sync
func StoreInitialSyncStartBlock(blockNumber string) {
	err := db.StoreInitialSyncStartBlock(blockNumber)
	if err != nil {
		configs.Logger.Error("Failed to store initial sync start block",
			zap.String("block", blockNumber),
			zap.Error(err))
	}
}
