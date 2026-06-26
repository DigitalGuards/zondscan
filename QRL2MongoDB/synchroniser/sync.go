package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Note: Token-related functions have been moved to token_sync.go:
// - ProcessTokensAfterInitialSync
// - ProcessTokenTransfersForBlock
// - InitializeTokenCollections
// - GetTokenSyncRange
// - StoreInitialSyncStartBlock

// initialSyncComplete is set to 1 after the initial block sync finishes.
// Mempool polling checks this flag and skips work while it is 0 to avoid
// competing with batch block fetches for RPC bandwidth.
var initialSyncComplete int32

// IsInitialSyncComplete returns true once the initial block sync has finished.
func IsInitialSyncComplete() bool {
	return atomic.LoadInt32(&initialSyncComplete) == 1
}

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

// getRPCDelay returns the configured delay duration with jitter.
// When bulkSync is true, the delay and jitter are reduced to 1/10th
// (with minimums of 5ms delay and 2ms jitter) to avoid overwhelming
// the node during batch block fetches.
func getRPCDelay(bulkSync bool) time.Duration {
	config := getSyncConfig()
	delay := config.RPCDelayMs
	jitter := config.RPCDelayJitter
	if bulkSync {
		delay = max(delay/10, 5)
		jitter = max(jitter/10, 2)
	}
	if jitter > 0 {
		delay += rand.Intn(jitter)
	}
	return time.Duration(delay) * time.Millisecond
}

// Sync starts the synchronization process. stopCh is closed by main.go on a
// termination signal; it is threaded into every background ticker goroutine
// started here so they stop accepting new work during graceful shutdown.
func Sync(stopCh <-chan struct{}) {
	var err error
	var nextBlock string
	var maxHex string

	// Ensure the token collections + their indexes exist before we start
	// processing blocks. Pre-Phase-2 this only ran via processInitialBlock,
	// which is gated on IsCollectionsExist() == false, but the configs init
	// path creates non-token collections (dailyTransactionsVolume etc), so
	// IsCollectionsExist returns true on every restart and the gate never
	// opens. Calling InitializeTokenCollections here is idempotent
	// (CreateMany no-ops on existing indexes) and guarantees the Phase 2
	// (contract, holder, tokenID) unique index actually gets created.
	if err := InitializeTokenCollections(); err != nil {
		configs.Logger.Error("Failed to initialize token collections at sync start",
			zap.Error(err))
		// Continue anyway: the indexes are about correctness, not liveness;
		// the syncer can still index blocks and we'll log the issue loudly.
	}

	// DB queries, no retry needed, these are local and don't fail transiently.
	nextBlock = db.GetLastKnownBlockNumber()
	if nextBlock == "0x0" {
		nextBlock = db.GetLatestBlockNumberFromDB()
		if nextBlock == "0x0" {
			configs.Logger.Info("No existing blocks found, starting from genesis")
		} else {
			configs.Logger.Info("Starting from latest block in DB",
				zap.String("block", nextBlock))
		}
	} else {
		configs.Logger.Info("Continuing from last known block",
			zap.String("block", nextBlock))
	}

	// Store the initial sync starting point for later token processing.
	// Only set if no existing start block is stored, to avoid redundant
	// full re-scans on every restart. Uses block 1 to ensure all tokens
	// are detected, including those created by factory contracts.
	existingStart := db.GetLastKnownBlockNumberFromInitialSync()
	if existingStart == "0x0" || existingStart == "" {
		StoreInitialSyncStartBlock("0x1")
	}

	nextBlock = utils.AddHexNumbers(nextBlock, "0x1")

	// Only the RPC call can fail transiently, retry with exponential backoff.
	for retries := 0; retries < 5; retries++ {
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
	initialSyncStart, tokenMaxHex := GetTokenSyncRange(lastSyncedBlock, maxHex)

	configs.Logger.Info("Processing token transfers for all synced blocks...",
		zap.String("from_block", initialSyncStart),
		zap.String("to_block", tokenMaxHex))

	// Process tokens using the dedicated token sync module
	ProcessTokensAfterInitialSync(initialSyncStart, tokenMaxHex)

	// Start auxiliary services after initial sync
	go func() {
		// Start wallet count sync
		configs.Logger.Info("Starting wallet count sync service...")
		db.StartWalletCountSync(stopCh)

		// Start contract reprocessing job
		configs.Logger.Info("Starting contract reprocessing service...")
		db.StartContractReprocessingJob(stopCh)
	}()

	// Signal that initial sync is done so mempool polling can begin
	atomic.StoreInt32(&initialSyncComplete, 1)
	configs.Logger.Info("Initial sync flag set, mempool polling enabled")

	configs.Logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion(stopCh)
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
// This is used to fix sync state issues when the normal update mechanism fails.
// Writes both block_number and block_number_int so the invariant that
// StoreLastKnownBlockNumber relies on (numeric $lt guard) isn't violated.
func forceUpdateSyncState(blockNumber string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, db.SyncStateCollection)

	// Use upsert to atomically update or insert the sync state
	_, err := syncColl.UpdateOne(
		ctx,
		bson.M{"_id": db.LastSyncedBlockID},
		bson.M{"$set": bson.M{
			"block_number":     blockNumber,
			"block_number_int": db.HexToInt64(blockNumber),
		}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		configs.Logger.Error("Failed to update sync state",
			zap.String("block", blockNumber),
			zap.Error(err))
	} else {
		configs.Logger.Info("Successfully updated sync state",
			zap.String("block", blockNumber))
	}
}

// processInitialBlock processes the genesis block and initializes collections
func processInitialBlock() {
	configs.Logger.Info("Processing genesis block")

	// Initialize token collections using the token sync module
	if err := InitializeTokenCollections(); err != nil {
		configs.Logger.Error("Failed to initialize token collections", zap.Error(err))
		// Continue anyway - we'll log the error but try to proceed
	}

	// Initialize validators
	configs.Logger.Info("Initializing validators")
	if err := syncValidators(); err != nil {
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

		// Reorg: block N (currentBlock) does not build on the stored block N-1
		// (parentBlockNum), so the stored N-1 is stale and must be removed.
		// Rollback deletes blocks with blockNumberInt > arg (strict $gt), so to
		// include N-1 in the deletion set we pass N-2 (parentBlockNum - 0x1).
		// Rollback then deletes every block >= N-1 (i.e. the stale N-1 and any
		// stragglers above it) and resets the sync state to N-2, so the
		// continuous loop re-syncs starting at N-1.
		rollbackTarget := utils.SubtractHexNumbers(parentBlockNum, "0x1")
		err = db.Rollback(rollbackTarget)
		if err != nil {
			configs.Logger.Error("Failed to rollback block",
				zap.String("rollback_target", rollbackTarget),
				zap.String("stale_block", parentBlockNum),
				zap.Error(err))
			// Rollback did not delete the stale block. Returning parentBlockNum
			// here would let the BlockExists guard skip the still-present N-1,
			// advance to N, re-detect the mismatch, and ping-pong forever
			// without ever repairing the data. Return currentBlock instead so
			// the next tick retries this same block (and the rollback) until it
			// succeeds.
			return currentBlock
		}
		// Resume from the rolled-back point: re-sync the stale block (N-1)
		// first so its parent linkage is rebuilt before N.
		return parentBlockNum
	}

	// Idempotency guard: if this block is already stored, skip BOTH the block
	// insert and ProcessTransactions together. InsertBlockDocument already
	// no-ops on an existing block, but ProcessTransactions used to run
	// unconditionally right after, duplicating transfer + transactionByAddress
	// rows (which have no unique index on txHash) on any re-process: retry
	// after a partial failure, node restart, or gap-fill overlap. Guarding both
	// with one BlockExists check keeps them consistent.
	//
	// This does NOT block the reorg re-sync path above: Rollback deletes the
	// stale block first, so BlockExists returns false on the subsequent
	// reprocess and the block is re-inserted correctly.
	if db.BlockExists(currentBlock) {
		configs.Logger.Info("Block already processed, skipping insert and transaction processing",
			zap.String("block", currentBlock),
			zap.String("hash", blockData.Result.Hash))
		// Still advance the sync state and return the next block so the caller
		// continues forward rather than re-checking the same block.
		db.StoreLastKnownBlockNumber(currentBlock)
		return utils.AddHexNumbers(currentBlock, "0x1")
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
