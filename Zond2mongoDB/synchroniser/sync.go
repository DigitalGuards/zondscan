package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// RPC delay constants (can be overridden via environment)
const (
	DefaultRPCDelayMs     = 50
	DefaultRPCDelayJitter = 26
)

// SyncConfig holds configurable sync settings
type SyncConfig struct {
	RPCDelayMs     int
	RPCDelayJitter int
}

// failedBlocks tracks blocks that failed during sync for later retry
var failedBlocks sync.Map

// getSyncConfig returns the sync configuration from environment or defaults
func getSyncConfig() SyncConfig {
	config := SyncConfig{
		RPCDelayMs:     DefaultRPCDelayMs,
		RPCDelayJitter: DefaultRPCDelayJitter,
	}

	if delay := os.Getenv("RPC_DELAY_MS"); delay != "" {
		if val, err := strconv.Atoi(delay); err == nil && val >= 0 {
			config.RPCDelayMs = val
		}
	}

	if jitter := os.Getenv("RPC_DELAY_JITTER_MS"); jitter != "" {
		if val, err := strconv.Atoi(jitter); err == nil && val >= 0 {
			config.RPCDelayJitter = val
		}
	}

	return config
}

// getRPCDelay returns the configured delay duration with jitter
func getRPCDelay() time.Duration {
	config := getSyncConfig()
	if config.RPCDelayJitter > 0 {
		return time.Duration(config.RPCDelayMs+rand.Intn(config.RPCDelayJitter)) * time.Millisecond
	}
	return time.Duration(config.RPCDelayMs) * time.Millisecond
}

// getRPCDelayForBulkSync returns a reduced delay for bulk sync operations
// When syncing many blocks, we want to go faster but still avoid overwhelming the node
func getRPCDelayForBulkSync() time.Duration {
	config := getSyncConfig()
	// Use 1/10th of the normal delay for bulk sync
	reducedDelay := config.RPCDelayMs / 10
	if reducedDelay < 5 {
		reducedDelay = 5 // Minimum 5ms to avoid completely overwhelming the node
	}
	if config.RPCDelayJitter > 0 {
		jitter := config.RPCDelayJitter / 10
		if jitter < 2 {
			jitter = 2
		}
		return time.Duration(reducedDelay+rand.Intn(jitter)) * time.Millisecond
	}
	return time.Duration(reducedDelay) * time.Millisecond
}

// Sync starts the synchronization process
func Sync() {
	var err error
	var nextBlock string
	var maxHex string

	// Retry getting initial sync points with exponential backoff
	for retries := 0; retries < 5; retries++ {
		// Try to get the last synced block
		nextBlock = db.GetLastKnownBlockNumber()
		if nextBlock == "0x0" {
			// If no last known block, try getting latest from DB
			nextBlock = db.GetLatestBlockNumberFromDB()
			if nextBlock == "0x0" {
				// If no blocks in DB, start from genesis
				nextBlock = "0x0"
				configs.Logger.Info("No existing blocks found, starting from genesis")
			} else {
				configs.Logger.Info("Starting from latest block in DB",
					zap.String("block", nextBlock))
			}
		} else {
			configs.Logger.Info("Continuing from last known block",
				zap.String("block", nextBlock))
		}

		// Store the initial sync starting point for later token processing
		if nextBlock == "0x0" {
			// If starting from genesis, record block 1 as the start
			storeInitialSyncStartBlock("0x1")
		} else {
			storeInitialSyncStartBlock(nextBlock)
		}

		nextBlock = utils.AddHexNumbers(nextBlock, "0x1")

		// Get latest block from network
		maxHex, err = rpc.GetLatestBlock()
		if err == nil {
			break
		}
		configs.Logger.Warn("Failed to get latest block, retrying...",
			zap.Error(err),
			zap.Int("retry", retries+1))
		time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
	}

	if err != nil {
		configs.Logger.Error("Failed to get latest block after retries", zap.Error(err))
		return
	}

	configs.Logger.Info("Starting sync from block number", zap.String("block", nextBlock))
	wg := sync.WaitGroup{}
	configs.Logger.Info("Latest block from network", zap.String("block", maxHex))

	// Initialize the producer semaphore
	producerSem = make(chan struct{}, MaxProducerConcurrency)

	// Create a buffered channel of read only channels, with length 32.
	producers := make(chan (<-chan Data), 32)
	configs.Logger.Info("Initialized producer channels")

	// Start the consumer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(producers)
	}()
	configs.Logger.Info("Started consumer process")

	// Increased batch size for faster initial sync
	batchSize := DefaultBatchSize
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(maxHex, nextBlock), utils.IntToHex(LargeSyncThreshold)) > 0 {
		batchSize = LargeBatchSize
	}

	// Start producers in correct order with larger batch size
	currentBlock := nextBlock
	for utils.CompareHexNumbers(currentBlock, maxHex) < 0 {
		endBlock := utils.AddHexNumbers(currentBlock, utils.IntToHex(batchSize))
		if utils.CompareHexNumbers(endBlock, maxHex) > 0 {
			endBlock = maxHex
		}
		producers <- producer(currentBlock, endBlock)
		configs.Logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", endBlock))
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()
	configs.Logger.Info("Initial sync completed successfully!")

	configs.Logger.Info("Calculating daily transaction volume...")
	db.GetDailyTransactionVolume()

	// Check the actual last known block after sync to ensure we have the right value
	lastSyncedBlock := db.GetLastKnownBlockNumber()
	configs.Logger.Info("Last synced block according to database",
		zap.String("block", lastSyncedBlock))

	// Get the latest block again to ensure we're using the most current value
	maxHex, err = rpc.GetLatestBlock()
	if err != nil {
		configs.Logger.Error("Failed to get latest block for token processing", zap.Error(err))
		// Continue with the old value if we can't get a new one
	} else {
		configs.Logger.Info("Updated latest block from network for token processing",
			zap.String("block", maxHex))
	}

	// Process token transfers for the entire range after the initial sync
	initialSyncStart := db.GetLastKnownBlockNumberFromInitialSync()
	if initialSyncStart == "0x0" {
		initialSyncStart = "0x1" // Start from block 1 if no initial sync start block is available
	}

	// Verify we aren't trying to process tokens beyond what's actually synced
	if utils.CompareHexNumbers(lastSyncedBlock, "0x0") > 0 &&
		utils.CompareHexNumbers(maxHex, lastSyncedBlock) > 0 {
		maxHex = lastSyncedBlock
		configs.Logger.Info("Limiting token processing to last synced block",
			zap.String("lastSyncedBlock", lastSyncedBlock))
	}

	configs.Logger.Info("Processing token transfers for all synced blocks...",
		zap.String("from_block", initialSyncStart),
		zap.String("to_block", maxHex))

	// Create a dedicated function to process tokens and call it directly
	processTokensAfterInitialSync(initialSyncStart, maxHex)

	// Start auxiliary services after initial sync
	go func() {
		// Start wallet count sync
		configs.Logger.Info("Starting wallet count sync service...")
		db.StartWalletCountSync()

		// Start contract reprocessing job
		configs.Logger.Info("Starting contract reprocessing service...")
		db.StartContractReprocessingJob()
	}()

	configs.Logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion()
}

// processTokensAfterInitialSync handles token transfer processing after the initial block sync is complete
func processTokensAfterInitialSync(initialSyncStart string, maxHex string) {
	configs.Logger.Info("Beginning post-sync token transfer processing",
		zap.String("from_block", initialSyncStart),
		zap.String("to_block", maxHex))

	// Get blocks with transactions only
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query for blocks that have at least one transaction
	filter := bson.M{
		"result.number": bson.M{
			"$gte": initialSyncStart,
			"$lte": maxHex,
		},
		"result.transactions.0": bson.M{"$exists": true}, // At least one transaction
	}

	// Only retrieve block numbers to keep memory usage low
	projection := bson.M{"result.number": 1, "_id": 0}

	cursor, err := configs.BlocksCollections.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		configs.Logger.Error("Failed to query blocks with transactions",
			zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var blocksWithTxs []string

	// Extract block numbers with transactions
	for cursor.Next(ctx) {
		var block struct {
			Result struct {
				Number string `bson:"number"`
			} `bson:"result"`
		}

		if err := cursor.Decode(&block); err != nil {
			configs.Logger.Error("Failed to decode block",
				zap.Error(err))
			continue
		}

		blocksWithTxs = append(blocksWithTxs, block.Result.Number)
	}

	if len(blocksWithTxs) == 0 {
		configs.Logger.Info("No blocks with transactions found in range")
		return
	}

	configs.Logger.Info("Found blocks with transactions to process",
		zap.Int("count", len(blocksWithTxs)))

	// Process token transfers for blocks with transactions in batches
	tokenBatchSize := 10
	totalProcessed := 0
	batchCounter := 0

	for i := 0; i < len(blocksWithTxs); i += tokenBatchSize {
		// Calculate end index for current batch
		end := i + tokenBatchSize
		if end > len(blocksWithTxs) {
			end = len(blocksWithTxs)
		}

		batchBlocks := blocksWithTxs[i:end]
		batchSize := len(batchBlocks)

		configs.Logger.Info("Processing token transfers batch",
			zap.Int("batch", batchCounter),
			zap.Int("size", batchSize))

		// Process each block in the batch
		for _, blockNumber := range batchBlocks {
			processTokenTransfersForBlock(blockNumber)
			totalProcessed++
		}

		configs.Logger.Info("Completed token transfer batch",
			zap.Int("batch", batchCounter),
			zap.Int("blocks_processed", batchSize),
			zap.Int("total_processed", totalProcessed))

		batchCounter++

		// Add a small delay between batches to prevent overwhelming the node
		time.Sleep(86 * time.Millisecond)
	}

	configs.Logger.Info("Completed token transfer processing for all blocks with transactions",
		zap.Int("total_blocks_processed", totalProcessed))
}

// findHighestProcessedBlock finds the highest block number that exists in the database
func findHighestProcessedBlock() string {
	// First try to get the last synced block from the database
	lastSyncedBlock, err := db.GetLastSyncedBlock()
	if err == nil && lastSyncedBlock != nil && lastSyncedBlock.Result.Number != "" {
		configs.Logger.Info("Using last synced block from sync state",
			zap.String("block", lastSyncedBlock.Result.Number))
		return lastSyncedBlock.Result.Number
	}

	// Fallback to the old method if the above fails
	// Get the latest block from the database
	latestBlock := db.GetLatestBlockFromDB()
	if latestBlock != nil && latestBlock.Result.Number != "" {
		return latestBlock.Result.Number
	}

	// Fallback to the last known block number
	return db.GetLastKnownBlockNumber()
}

// forceUpdateSyncState directly updates the sync state without conditions
// This is used to fix sync state issues when the normal update mechanism fails
func forceUpdateSyncState(blockNumber string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, db.SyncStateCollection)

	// First, try to delete the existing document to avoid duplicate key errors
	_, err := syncColl.DeleteOne(ctx, bson.M{"_id": db.LastSyncedBlockID})
	if err != nil {
		configs.Logger.Warn("Failed to delete existing sync state document",
			zap.Error(err))
		// Continue anyway - the document might not exist
	}

	// Now insert a new document with the updated block number
	_, err = syncColl.InsertOne(ctx, bson.M{
		"_id":          db.LastSyncedBlockID,
		"block_number": blockNumber,
	})

	if err != nil {
		configs.Logger.Error("Failed to force update sync state",
			zap.String("block", blockNumber),
			zap.Error(err))

		// As a fallback, try a regular update
		_, updateErr := syncColl.UpdateOne(
			ctx,
			bson.M{"_id": db.LastSyncedBlockID},
			bson.M{"$set": bson.M{"block_number": blockNumber}},
			options.Update().SetUpsert(true),
		)

		if updateErr != nil {
			configs.Logger.Error("Fallback update also failed",
				zap.String("block", blockNumber),
				zap.Error(updateErr))
		} else {
			configs.Logger.Info("Successfully forced update of sync state using fallback method",
				zap.String("block", blockNumber))
		}
	} else {
		configs.Logger.Info("Successfully forced update of sync state",
			zap.String("block", blockNumber))
	}
}

// processInitialBlock processes the genesis block and initializes collections
func processInitialBlock() {
	configs.Logger.Info("Processing genesis block")

	// Initialize collections that need special handling
	configs.Logger.Info("Initializing token collections")

	// Initialize token transfers collection
	err := db.InitializeTokenTransfersCollection()
	if err != nil {
		configs.Logger.Error("Failed to initialize token transfers collection", zap.Error(err))
		// Continue anyway - we'll log the error but try to proceed
	} else {
		configs.Logger.Info("Successfully initialized token transfers collection")
	}

	// Initialize token balances collection
	err = db.InitializeTokenBalancesCollection()
	if err != nil {
		configs.Logger.Error("Failed to initialize token balances collection", zap.Error(err))
		// Continue anyway - we'll log the error but try to proceed
	} else {
		configs.Logger.Info("Successfully initialized token balances collection")
	}

	// Initialize validators
	configs.Logger.Info("Initializing validators")
	err = syncValidators()
	if err != nil {
		configs.Logger.Error("Failed to initialize validators", zap.Error(err))
	} else {
		configs.Logger.Info("Successfully initialized validators")
	}

	// Get block 0
	genesisBlock, err := rpc.GetBlockByNumberMainnet("0x0")
	if err != nil {
		configs.Logger.Error("Failed to get genesis block",
			zap.Error(err))
		return
	}

	// Update tx status in block 0
	db.UpdateTransactionStatuses(genesisBlock)

	// Insert block document
	blocksCollection := configs.GetCollection(configs.DB, "blocks")
	ctx := context.Background()
	_, err = blocksCollection.InsertOne(ctx, genesisBlock)
	if err != nil {
		configs.Logger.Error("Failed to insert genesis block",
			zap.Error(err))
		return
	}

	// Process transactions
	db.ProcessTransactions(*genesisBlock)

	db.StoreLastKnownBlockNumber("0x0")
	configs.Logger.Info("Genesis block processed successfully")
}

// processSubsequentBlocks processes a single block and returns the next block to process
func processSubsequentBlocks(currentBlock string) string {
	// Get the block data from the node with retry logic
	var blockData *models.ZondDatabaseBlock
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		blockData, err = rpc.GetBlockByNumberMainnet(currentBlock)
		if err == nil && blockData != nil && blockData.Result.ParentHash != "" {
			break // Success
		}

		if attempt < maxRetries {
			backoffDelay := time.Duration(attempt*500) * time.Millisecond
			configs.Logger.Warn("Block fetch failed in processSubsequentBlocks, retrying",
				zap.String("block", currentBlock),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoffDelay),
				zap.Error(err))
			time.Sleep(backoffDelay)
		}
	}

	if err != nil {
		configs.Logger.Error("Failed to get block data after retries",
			zap.String("block", currentBlock),
			zap.Int("max_retries", maxRetries),
			zap.Error(err))
		trackFailedBlock(currentBlock, err)
		// Return empty string to signal failure - caller should handle retry
		return ""
	}

	if blockData == nil || blockData.Result.ParentHash == "" {
		configs.Logger.Error("Invalid block data received after retries",
			zap.String("block", currentBlock))
		trackFailedBlock(currentBlock, fmt.Errorf("invalid block data: nil or missing parent hash"))
		return ""
	}

	// Get the parent block's hash from our DB
	parentBlockNum := utils.SubtractHexNumbers(currentBlock, "0x1")
	dbParentHash := db.GetLatestBlockHashHeaderFromDB(parentBlockNum)

	// If this is not the genesis block and we don't have the parent, we need to sync the parent first
	if parentBlockNum != "0x0" && dbParentHash == "" {
		configs.Logger.Info("Missing parent block, syncing parent first",
			zap.String("current_block", currentBlock),
			zap.String("parent_block", parentBlockNum))
		return parentBlockNum
	}

	// For non-genesis blocks, verify parent hash
	if parentBlockNum != "0x0" && blockData.Result.ParentHash != dbParentHash {
		configs.Logger.Warn("Parent hash mismatch detected",
			zap.String("block", currentBlock),
			zap.String("expected_parent", dbParentHash),
			zap.String("actual_parent", blockData.Result.ParentHash))

		// Roll back one block and try again
		err = db.Rollback(currentBlock)
		if err != nil {
			configs.Logger.Error("Failed to rollback block",
				zap.String("block", currentBlock),
				zap.Error(err))
		}
		return parentBlockNum
	}

	// Process the block
	db.InsertBlockDocument(*blockData)
	db.ProcessTransactions(*blockData)

	// Update any pending transactions that are now mined in this block
	if err := UpdatePendingTransactionsInBlock(blockData); err != nil {
		configs.Logger.Error("Failed to update pending transactions in block",
			zap.String("block", blockData.Result.Number),
			zap.Error(err))
		// Don't return error to avoid blocking block processing
	}

	configs.Logger.Info("Block processed successfully",
		zap.String("block", currentBlock),
		zap.String("hash", blockData.Result.Hash))

	// Update sync state after successful processing
	db.StoreLastKnownBlockNumber(currentBlock)

	// Return next block number
	return utils.AddHexNumbers(currentBlock, "0x1")
}

// processTokenTransfersForBlock processes token transfers in a block
func processTokenTransfersForBlock(blockNumber string) {
	// Get block from database to get timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	configs.Logger.Info("Starting token transfer processing for block",
		zap.String("blockNumber", blockNumber))

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

	// Skip token transfer processing if block has no transactions
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

// storeInitialSyncStartBlock stores the block number that was used as the starting point
// for the initial sync. This is used for token transfer processing after initial sync.
func storeInitialSyncStartBlock(blockNumber string) {
	err := db.StoreInitialSyncStartBlock(blockNumber)
	if err != nil {
		configs.Logger.Error("Failed to store initial sync start block",
			zap.String("block", blockNumber),
			zap.Error(err))
	}
}
