package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
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
func Sync(stopCh <-chan struct{}) error {
	var err error
	var nextBlock string
	var maxHex string
	if err := db.ValidateBlockIngestionMigration(); err != nil {
		configs.Logger.Error("Block ingestion migration gate failed; synchronization is disabled until companion rows are explicitly reindexed",
			zap.Error(err))
		return err
	}

	// Token work has its own durable per-block claim. Normalize and validate the
	// queue synchronously so an unknown state fails startup instead of hiding a
	// block forever, then keep the drainer alive for quiet-head retries.
	if err := initializePendingTokenBlockQueue(); err != nil {
		return fmt.Errorf("initialize durable token block queue: %w", err)
	}
	StartPendingTokenBlockWorker(stopCh)

	// The producer loop starts at lastKnown+1, so genesis is checked through
	// the same durable path on every startup. This also resumes a pending
	// companion write left by a crash before its completion marker.
	var genErr error
	for retries := 0; retries < 5; retries++ {
		var genesisBlock *models.ZondDatabaseBlock
		genesisBlock, genErr = rpc.GetBlockByNumberMainnet("0x0")
		if genErr == nil {
			genErr = ingestFetchedBlock("0x0", genesisBlock)
		}
		if genErr == nil {
			configs.Logger.Info("Genesis block reached durable ingestion completion")
			break
		}
		configs.Logger.Warn("Failed to ingest genesis block, retrying",
			zap.Error(genErr),
			zap.Int("retry", retries+1))
		time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
	}
	if genErr != nil {
		configs.Logger.Error("Failed to ingest genesis block after retries; canonical sync will remain at its confirmed prefix",
			zap.Error(genErr))
	}

	// The durable sync state is the only startup cursor. A block row can exist
	// with ingestionState=pending after a crash, so deriving the cursor from the
	// numerically latest row would skip its companion replay.
	nextBlock = db.GetLastKnownBlockNumber()
	if nextBlock == "0x0" {
		configs.Logger.Info("Starting from the durable genesis sync cursor")
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
		return fmt.Errorf("get latest block after retries: %w", err)
	}

	configs.Logger.Info("Starting sync from block number", zap.String("block", nextBlock))
	wg := sync.WaitGroup{}
	configs.Logger.Info("Latest block from network", zap.String("block", maxHex))

	producerGroup := newBlockProducerGroup(MaxProducerConcurrency)

	// Create a buffered channel of read only channels, with length 32.
	producers := make(chan (<-chan Data), 32)
	initialBatchCtx, cancelInitialBatch := context.WithCancel(context.Background())
	configs.Logger.Info("Initialized producer channels")

	// Start the consumer.
	var initialBatchReport consumerReport
	wg.Add(1)
	go func() {
		defer wg.Done()
		initialBatchReport = consumeBatches(producers, cancelInitialBatch)
	}()
	configs.Logger.Info("Started consumer process")

	// Increased batch size for faster initial sync
	batchSize := DefaultBatchSize
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(maxHex, nextBlock), utils.IntToHex(LargeSyncThreshold)) > 0 {
		batchSize = LargeBatchSize
	}

	// Start producers in correct order with larger batch size
	currentBlock := nextBlock
scheduleInitialBatches:
	for utils.CompareHexNumbers(currentBlock, maxHex) < 0 {
		select {
		case <-initialBatchCtx.Done():
			break scheduleInitialBatches
		default:
		}
		endBlock := utils.AddHexNumbers(currentBlock, utils.IntToHex(batchSize))
		if utils.CompareHexNumbers(endBlock, maxHex) > 0 {
			endBlock = maxHex
		}
		producerChan := producerGroup.producerWithContext(initialBatchCtx, currentBlock, endBlock)
		select {
		case producers <- producerChan:
		case <-initialBatchCtx.Done():
			break scheduleInitialBatches
		}
		configs.Logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", endBlock))
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()
	cancelInitialBatch()
	producerGroup.wait()
	if initialBatchReport.err != nil {
		configs.Logger.Error("Initial batch sync stopped at its confirmed canonical prefix",
			zap.Int("highest_processed_block", initialBatchReport.highestProcessedBlock),
			zap.Error(initialBatchReport.err))
	} else {
		configs.Logger.Info("Initial sync completed successfully!")
	}

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

	// Start auxiliary services after initial sync. Their goroutines are
	// registered so shutdown cannot release the writer lease while one is
	// still mutating MongoDB.
	configs.Logger.Info("Starting wallet count sync service...")
	StartWalletCountSync(stopCh)

	configs.Logger.Info("Starting contract reprocessing service...")
	StartContractReprocessingJob(stopCh)

	// Signal that initial sync is done so mempool polling can begin
	atomic.StoreInt32(&initialSyncComplete, 1)
	configs.Logger.Info("Initial sync flag set, mempool polling enabled")

	configs.Logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion(stopCh)
	return nil
}

// findHighestProcessedBlock returns the sync-state block number.
// db.GetLastKnownBlockNumber never fails (it returns "0x0" when the sync
// state is missing), so the historical fallbacks to GetLatestBlockFromDB
// and a second sync-state read were unreachable and have been removed.
func findHighestProcessedBlock() string {
	block := db.GetLastKnownBlockNumber()
	configs.Logger.Info("Using last synced block from sync state",
		zap.String("block", block))
	return block
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
			"block_number_int": utils.HexToInt64(blockNumber),
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

// processSubsequentBlocks processes a single block and returns the next block to process
func processSubsequentBlocks(currentBlock string) string {
	// Get the block data from the node with retry logic
	var blockData *models.ZondDatabaseBlock
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		blockData, err = rpc.GetBlockByNumberMainnet(currentBlock)
		if err == nil {
			err = validateFetchedBlock(currentBlock, blockData)
		}
		if err == nil {
			err = db.UpdateTransactionStatuses(blockData)
		}
		if err == nil {
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

	// Serialize the complete indexed mutation with gap-fill, batch ingestion,
	// and rollback. Parent linkage is re-read under the same lock so a rollback
	// cannot remove it between validation and child insertion.
	lockChainMutation()
	defer chainMutationMu.Unlock()

	currentNumber, parseErr := parseBatchBlockNumber(currentBlock)
	if parseErr != nil {
		trackFailedBlock(currentBlock, parseErr)
		return ""
	}

	if currentNumber > 0 {
		parentBlockNum := utils.IntToHex(currentNumber - 1)
		dbParentHash, parentErr := db.GetCanonicalBlockHash(parentBlockNum)
		if parentErr != nil {
			if errors.Is(parentErr, db.ErrCanonicalBlockNotFound) ||
				errors.Is(parentErr, db.ErrBlockWriteUnresolved) {
				configs.Logger.Info("Parent block is missing or incomplete; repairing it first",
					zap.String("current_block", currentBlock),
					zap.String("parent_block", parentBlockNum))
				return parentBlockNum
			}
			configs.Logger.Error("Failed to read canonical parent identity",
				zap.String("current_block", currentBlock),
				zap.String("parent_block", parentBlockNum),
				zap.Error(parentErr))
			trackFailedBlock(currentBlock, parentErr)
			return ""
		}

		if !strings.EqualFold(blockData.Result.ParentHash, dbParentHash) {
			configs.Logger.Warn("Parent hash mismatch detected",
				zap.String("block", currentBlock),
				zap.String("expected_parent", dbParentHash),
				zap.String("actual_parent", blockData.Result.ParentHash))
			if currentNumber == 1 {
				conflictErr := fmt.Errorf("block %s does not build on immutable genesis", currentBlock)
				trackFailedBlock(currentBlock, conflictErr)
				return ""
			}

			// Reorg: block N (currentBlock) does not build on the stored block N-1
			// (parentBlockNum), so the stored N-1 is stale and must be removed.
			// Rollback deletes blocks with blockNumberInt > arg (strict $gt), so to
			// include N-1 in the deletion set we pass N-2 (parentBlockNum - 0x1).
			// Rollback then deletes every block >= N-1 (i.e. the stale N-1 and any
			// stragglers above it) and resets the sync state to N-2, so the
			// continuous loop re-syncs starting at N-1.
			rollbackTarget := utils.IntToHex(currentNumber - 2)
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
	}

	confirmed, ingestErr := ingestPreparedBatchLocked([]batchBlock{{
		block:  *blockData,
		number: currentNumber,
	}}, nil)
	if ingestErr != nil {
		configs.Logger.Error("Durable block ingestion failed",
			zap.String("block", currentBlock),
			zap.Error(ingestErr))
		trackFailedBlock(currentBlock, ingestErr)
		return ""
	}
	if len(confirmed) != 1 || confirmed[0] != currentNumber {
		unresolvedErr := fmt.Errorf("block %s did not reach durable companion completion", currentBlock)
		trackFailedBlock(currentBlock, unresolvedErr)
		return ""
	}

	configs.Logger.Info("Block processed successfully",
		zap.String("block", currentBlock),
		zap.String("hash", blockData.Result.Hash))

	// Return next block number
	return utils.AddHexNumbers(currentBlock, "0x1")
}
