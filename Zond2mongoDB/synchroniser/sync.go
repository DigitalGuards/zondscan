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
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Batch size constants for consistent use across sync methods
const (
	// DefaultBatchSize is the standard batch size for normal sync operations
	DefaultBatchSize = 64

	// LargeBatchSize is used when syncing a large number of blocks (>1000)
	LargeBatchSize = 128

	// BatchSyncThreshold is the number of blocks behind after which we switch to batch sync
	BatchSyncThreshold = 64

	// LargeSyncThreshold is the number of blocks that triggers using the larger batch size
	LargeSyncThreshold = 1000 // 0x3e8 in hex

	MaxProducerConcurrency = 8 // Limit concurrent block fetching goroutines

	// Default RPC delay settings (can be overridden via environment)
	DefaultRPCDelayMs    = 50
	DefaultRPCDelayJitter = 26

	// Gap detection constants
	MaxGapDetectionBlocks = 1000 // Maximum blocks to check for gaps
	GapRetryAttempts      = 3    // Number of retry attempts for filling gaps
)

// Semaphore to limit concurrent producer goroutines
var producerSem chan struct{}

type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

// FailedBlock tracks blocks that failed to sync with retry information
type FailedBlock struct {
	BlockNumber string
	Attempts    int
	LastError   error
	LastAttempt time.Time
}

// SyncConfig holds configurable sync settings
type SyncConfig struct {
	RPCDelayMs    int
	RPCDelayJitter int
}

// failedBlocks tracks blocks that failed during sync for later retry
var failedBlocks sync.Map

// getSyncConfig returns the sync configuration from environment or defaults
func getSyncConfig() SyncConfig {
	config := SyncConfig{
		RPCDelayMs:    DefaultRPCDelayMs,
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

// trackFailedBlock records a failed block for later retry
func trackFailedBlock(blockNumber string, err error) {
	existing, loaded := failedBlocks.Load(blockNumber)
	if loaded {
		failed := existing.(*FailedBlock)
		failed.Attempts++
		failed.LastError = err
		failed.LastAttempt = time.Now()
	} else {
		failedBlocks.Store(blockNumber, &FailedBlock{
			BlockNumber: blockNumber,
			Attempts:    1,
			LastError:   err,
			LastAttempt: time.Now(),
		})
	}
	configs.Logger.Warn("Tracked failed block for retry",
		zap.String("block", blockNumber),
		zap.Error(err))
}

// clearFailedBlock removes a block from the failed tracking after successful sync
func clearFailedBlock(blockNumber string) {
	failedBlocks.Delete(blockNumber)
}

// detectGaps finds missing blocks in the database within a range
func detectGaps(fromBlock, toBlock string) []string {
	configs.Logger.Info("Detecting gaps in block range",
		zap.String("from", fromBlock),
		zap.String("to", toBlock))

	fromNum := utils.HexToInt(fromBlock).Int64()
	toNum := utils.HexToInt(toBlock).Int64()

	// Limit the range to prevent memory issues
	if toNum-fromNum > MaxGapDetectionBlocks {
		fromNum = toNum - MaxGapDetectionBlocks
		fromBlock = utils.IntToHex(int(fromNum))
		configs.Logger.Info("Limiting gap detection range",
			zap.String("adjusted_from", fromBlock))
	}

	// Get all existing block numbers in the range
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"result.number": bson.M{
			"$gte": fromBlock,
			"$lte": toBlock,
		},
	}

	projection := bson.M{"result.number": 1, "_id": 0}
	cursor, err := configs.BlocksCollections.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		configs.Logger.Error("Failed to query blocks for gap detection", zap.Error(err))
		return nil
	}
	defer cursor.Close(ctx)

	existingBlocks := make(map[int64]bool)
	for cursor.Next(ctx) {
		var block struct {
			Result struct {
				Number string `bson:"number"`
			} `bson:"result"`
		}
		if err := cursor.Decode(&block); err != nil {
			continue
		}
		blockNum := utils.HexToInt(block.Result.Number).Int64()
		existingBlocks[blockNum] = true
	}

	// Find missing blocks
	var gaps []string
	for i := fromNum; i <= toNum; i++ {
		if !existingBlocks[i] {
			gaps = append(gaps, utils.IntToHex(int(i)))
		}
	}

	if len(gaps) > 0 {
		configs.Logger.Warn("Found block gaps",
			zap.Int("gap_count", len(gaps)),
			zap.String("from", fromBlock),
			zap.String("to", toBlock))
	}

	return gaps
}

// fillGaps attempts to sync missing blocks
func fillGaps(gaps []string) int {
	if len(gaps) == 0 {
		return 0
	}

	configs.Logger.Info("Attempting to fill block gaps",
		zap.Int("gap_count", len(gaps)))

	filled := 0
	for _, blockNum := range gaps {
		// Check if we've already tried this block too many times
		if existing, ok := failedBlocks.Load(blockNum); ok {
			failed := existing.(*FailedBlock)
			if failed.Attempts >= GapRetryAttempts {
				configs.Logger.Warn("Skipping block after max retry attempts",
					zap.String("block", blockNum),
					zap.Int("attempts", failed.Attempts))
				continue
			}
		}

		// Add RPC delay to prevent overwhelming the node
		time.Sleep(getRPCDelay())

		// Fetch and insert the block
		data, err := rpc.GetBlockByNumberMainnet(blockNum)
		if err != nil {
			trackFailedBlock(blockNum, err)
			configs.Logger.Error("Failed to fetch block for gap fill",
				zap.String("block", blockNum),
				zap.Error(err))
			continue
		}

		if data == nil || data.Result.ParentHash == "" {
			trackFailedBlock(blockNum, fmt.Errorf("invalid block data"))
			configs.Logger.Error("Invalid block data for gap fill",
				zap.String("block", blockNum))
			continue
		}

		// Insert the block
		db.UpdateTransactionStatuses(data)
		db.InsertBlockDocument(*data)
		db.ProcessTransactions(*data)

		// Update pending transactions
		if err := UpdatePendingTransactionsInBlock(data); err != nil {
			configs.Logger.Error("Failed to update pending transactions during gap fill",
				zap.String("block", blockNum),
				zap.Error(err))
		}

		clearFailedBlock(blockNum)
		filled++

		configs.Logger.Info("Filled block gap",
			zap.String("block", blockNum))
	}

	configs.Logger.Info("Gap fill completed",
		zap.Int("filled", filled),
		zap.Int("total_gaps", len(gaps)))

	return filled
}

// batchSync handles syncing multiple blocks in parallel
func batchSync(fromBlock string, toBlock string) string {
	// Sanity check to prevent backwards sync
	if utils.CompareHexNumbers(fromBlock, toBlock) >= 0 {
		configs.Logger.Error("Invalid block range for batch sync",
			zap.String("from_block", fromBlock),
			zap.String("to_block", toBlock))
		return fromBlock
	}

	configs.Logger.Info("Starting batch sync",
		zap.String("from_block", fromBlock),
		zap.String("to_block", toBlock))

	// Check if the last known block is already higher than our starting point
	// This prevents duplicate processing if another process has already synced these blocks
	lastKnownBlock := db.GetLastKnownBlockNumber()
	if utils.CompareHexNumbers(lastKnownBlock, fromBlock) >= 0 {
		configs.Logger.Info("Skipping batch sync as blocks have already been processed",
			zap.String("last_known_block", lastKnownBlock),
			zap.String("requested_from_block", fromBlock))

		// Return the higher of the two values to continue from there
		if utils.CompareHexNumbers(lastKnownBlock, toBlock) >= 0 {
			return toBlock
		}
		return lastKnownBlock
	}

	wg := sync.WaitGroup{}

	// Initialize the producer semaphore
	producerSem = make(chan struct{}, MaxProducerConcurrency)

	// Create buffered channel for producers
	producers := make(chan (<-chan Data), 32)

	// Start the consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(producers)
	}()

	// Use larger batch size when far behind
	batchSize := DefaultBatchSize
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(toBlock, fromBlock), utils.IntToHex(LargeSyncThreshold)) > 0 {
		batchSize = LargeBatchSize
	}

	// Start producers in batches with retry logic
	currentBlock := fromBlock
	lastSuccessfulBatch := fromBlock

	for utils.CompareHexNumbers(currentBlock, toBlock) < 0 {
		endBlock := utils.AddHexNumbers(currentBlock, utils.IntToHex(batchSize))
		if utils.CompareHexNumbers(endBlock, toBlock) > 0 {
			endBlock = toBlock
		}

		// Retry logic for producer
		var producerChan <-chan Data
		for retries := 0; retries < 3; retries++ {
			producerChan = producer(currentBlock, endBlock)
			if producerChan != nil {
				break
			}
			configs.Logger.Warn("Failed to create producer, retrying...",
				zap.String("from", currentBlock),
				zap.String("to", endBlock),
				zap.Int("retry", retries+1))
			time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
		}

		if producerChan == nil {
			configs.Logger.Error("Failed to create producer after retries",
				zap.String("from", currentBlock),
				zap.String("to", endBlock))
			return currentBlock
		}

		producers <- producerChan
		configs.Logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", endBlock))

		lastSuccessfulBatch = endBlock
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()

	// After batch sync completes, verify what the actual last synced block is
	lastKnownBlock = db.GetLastKnownBlockNumber()
	configs.Logger.Info("batchSync completed",
		zap.String("requested_to_block", toBlock),
		zap.String("last_successful_batch", lastSuccessfulBatch),
		zap.String("db_last_known_block", lastKnownBlock))

	if utils.CompareHexNumbers(lastSuccessfulBatch, lastKnownBlock) > 0 {
		configs.Logger.Info("Forcing update of sync state to latest processed block",
			zap.String("from", lastKnownBlock),
			zap.String("to", lastSuccessfulBatch))

		// Force update the sync state by directly setting it without conditions
		forceUpdateSyncState(lastSuccessfulBatch)

		// Update our local variable to reflect the change
		lastKnownBlock = lastSuccessfulBatch
	}

	// Process all token transfers once after all batches are completed
	db.ProcessTokenTransfersFromTransactions()

	configs.Logger.Info("Final sync state verification")
	highestBlock := findHighestProcessedBlock()
	if utils.CompareHexNumbers(highestBlock, lastKnownBlock) > 0 {
		configs.Logger.Info("Found higher processed block than current sync state",
			zap.String("current_sync_state", lastKnownBlock),
			zap.String("highest_processed_block", highestBlock))
		forceUpdateSyncState(highestBlock)
		lastKnownBlock = highestBlock
	}

	// Detect and fill any gaps that occurred during batch sync
	configs.Logger.Info("Running gap detection after batch sync")
	gaps := detectGaps(fromBlock, toBlock)
	if len(gaps) > 0 {
		configs.Logger.Warn("Found gaps in batch sync, attempting to fill",
			zap.Int("gap_count", len(gaps)))
		filled := fillGaps(gaps)
		if filled > 0 {
			configs.Logger.Info("Filled gaps during batch sync",
				zap.Int("filled", filled),
				zap.Int("remaining", len(gaps)-filled))
			// Update sync state after filling gaps
			newHighest := findHighestProcessedBlock()
			if utils.CompareHexNumbers(newHighest, lastKnownBlock) > 0 {
				forceUpdateSyncState(newHighest)
				lastKnownBlock = newHighest
			}
		}
	}

	if utils.CompareHexNumbers(lastKnownBlock, "0x0") > 0 {
		return lastKnownBlock
	}
	return lastSuccessfulBatch
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
	// Use the initialSyncStart from when we began, but verify against the actual
	// lastSyncedBlock to make sure we don't process more than what's synced
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

func consumer(ch <-chan (<-chan Data)) {
	var wg sync.WaitGroup
	var syncMutex sync.Mutex // Mutex for synchronizing block updates

	// Track the highest block number processed using atomic operations to prevent race conditions
	var highestProcessedBlock int64 = 0

	// Track all processed blocks for gap detection
	var processedBlocksMutex sync.Mutex
	processedBlocks := make([]int, 0)

	for producer := range ch {
		wg.Add(1)
		go func(p <-chan Data) {
			defer wg.Done()
			for data := range p {
				// Only process if there's data to process
				if len(data.blockData) > 0 {
					db.InsertManyBlockDocuments(data.blockData)
					configs.Logger.Info("Inserted block batch",
						zap.Int("count", len(data.blockData)))

					for x := 0; x < len(data.blockNumbers); x++ {
						db.ProcessTransactions(data.blockData[x])
					}
					configs.Logger.Info("Processed transactions for blocks",
						zap.Ints("block_numbers", data.blockNumbers))

					// Track processed blocks for gap detection (thread-safe)
					processedBlocksMutex.Lock()
					processedBlocks = append(processedBlocks, data.blockNumbers...)
					processedBlocksMutex.Unlock()

					// Store the last block number from this batch
					if len(data.blockNumbers) > 0 {
						syncMutex.Lock()
						lastBlock := utils.IntToHex(data.blockNumbers[len(data.blockNumbers)-1])
						db.StoreLastKnownBlockNumber(lastBlock)
						syncMutex.Unlock()
					}

					// Track the highest block number processed using atomic compare-and-swap
					for _, blockNum := range data.blockNumbers {
						blockNum64 := int64(blockNum)
						for {
							current := atomic.LoadInt64(&highestProcessedBlock)
							if blockNum64 <= current {
								break
							}
							if atomic.CompareAndSwapInt64(&highestProcessedBlock, current, blockNum64) {
								break
							}
						}
					}
				}
			}
		}(producer)
	}
	wg.Wait()

	// After all batches are processed, update the sync state with the highest block number
	highest := atomic.LoadInt64(&highestProcessedBlock)
	if highest > 0 {
		highestBlockHex := utils.IntToHex(int(highest))
		configs.Logger.Info("Updating sync state with highest processed block after batch processing",
			zap.String("block", highestBlockHex))
		forceUpdateSyncState(highestBlockHex)

		// Check for gaps in the processed blocks
		processedBlocksMutex.Lock()
		if len(processedBlocks) > 1 {
			sort.Ints(processedBlocks)
			minBlock := processedBlocks[0]
			maxBlock := processedBlocks[len(processedBlocks)-1]

			// If we processed fewer blocks than the range suggests, there might be gaps
			expectedCount := maxBlock - minBlock + 1
			if len(processedBlocks) < expectedCount {
				configs.Logger.Warn("Potential gaps detected during batch processing",
					zap.Int("expected_blocks", expectedCount),
					zap.Int("processed_blocks", len(processedBlocks)),
					zap.Int("min_block", minBlock),
					zap.Int("max_block", maxBlock))
			}
		}
		processedBlocksMutex.Unlock()
	}
}

func producer(start string, end string) <-chan Data {
	// Create a channel which we will send our data.
	Datas := make(chan Data, 32)

	var blockData []interface{}
	var blockNumbers []int

	// Start the goroutine that produces data.
	go func(ch chan<- Data) {
		// Acquire a token from the producer semaphore
		producerSem <- struct{}{}
		// Ensure the token is released when this goroutine finishes
		defer func() {
			<-producerSem
			close(ch) // Close the channel when done producing
		}()

		// Produce data.
		currentBlock := start
		for utils.CompareHexNumbers(currentBlock, end) < 0 {
			// Check if this block already exists in the database
			if db.BlockExists(currentBlock) {
				configs.Logger.Debug("Block already exists in database, skipping",
					zap.String("block", currentBlock))
				currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
				continue
			}

			// Add configurable delay between RPC calls to prevent overwhelming the node
			time.Sleep(getRPCDelay())

			// Try to fetch block with retry logic
			var data *models.ZondDatabaseBlock
			var err error
			maxRetries := 3

			for attempt := 1; attempt <= maxRetries; attempt++ {
				data, err = rpc.GetBlockByNumberMainnet(currentBlock)
				if err == nil && data != nil && data.Result.ParentHash != "" {
					break // Success
				}

				if attempt < maxRetries {
					backoffDelay := time.Duration(attempt*100) * time.Millisecond
					configs.Logger.Warn("Block fetch failed, retrying",
						zap.String("block", currentBlock),
						zap.Int("attempt", attempt),
						zap.Duration("backoff", backoffDelay),
						zap.Error(err))
					time.Sleep(backoffDelay)
				}
			}

			if err != nil {
				trackFailedBlock(currentBlock, err)
				configs.Logger.Error("Failed to get block data after retries",
					zap.String("block", currentBlock),
					zap.Int("max_retries", maxRetries),
					zap.Error(err))
				currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
				continue
			}

			if data == nil || data.Result.ParentHash == "" {
				trackFailedBlock(currentBlock, fmt.Errorf("invalid block data: nil or missing parent hash"))
				configs.Logger.Error("Invalid block data received",
					zap.String("block", currentBlock))
				currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
				continue
			}

			// Success - clear any previous failure tracking
			clearFailedBlock(currentBlock)

			db.UpdateTransactionStatuses(data)
			blockData = append(blockData, *data)
			blockNumbers = append(blockNumbers, int(utils.HexToInt(currentBlock).Int64()))
			currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
		}
		if len(blockData) > 0 {
			ch <- Data{blockData: blockData, blockNumbers: blockNumbers}
		}
	}(Datas)

	return Datas
}

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

func runPeriodicTask(task func(), interval time.Duration, taskName string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				configs.Logger.Error("Recovered from panic in periodic task",
					zap.String("task", taskName),
					zap.Any("error", r))
				// Restart the task after a short delay
				time.Sleep(5 * time.Second)
				runPeriodicTask(task, interval, taskName)
			}
		}()

		configs.Logger.Info("Starting periodic task",
			zap.String("task", taskName),
			zap.Duration("interval", interval))

		// Run immediately on start
		runTaskWithRetry(task, taskName)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			runTaskWithRetry(task, taskName)
		}
	}()
}

func runTaskWithRetry(task func(), taskName string) {
	maxAttempts := 5
	attempt := 1

	for attempt <= maxAttempts {
		configs.Logger.Info("Running periodic task",
			zap.String("task", taskName),
			zap.Int("attempt", attempt))

		func() {
			defer func() {
				if r := recover(); r != nil {
					configs.Logger.Error("Task panicked",
						zap.String("task", taskName),
						zap.Any("error", r))
				}
			}()
			task()
			// Only mark as complete if no panic occurred
			configs.Logger.Info("Completed periodic task",
				zap.String("task", taskName),
				zap.Int("attempt", attempt))
			attempt = maxAttempts + 1 // Exit loop on success
		}()

		if attempt <= maxAttempts {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			configs.Logger.Warn("Retrying task after failure",
				zap.String("task", taskName),
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay))
			time.Sleep(delay)
			attempt++
		}
	}
}

func processBlockPeriodically() {
	configs.Logger.Info("Starting block processing check")

	// Initialize collections if they don't exist
	if !db.IsCollectionsExist() {
		processInitialBlock()
		return
	}

	// Process the latest block
	latestBlock, err := rpc.GetLatestBlock()
	if err != nil {
		configs.Logger.Error("Failed to get latest block", zap.Error(err))
		return
	}

	lastProcessedBlock := db.GetLastKnownBlockNumber()
	if lastProcessedBlock == "0x0" {
		configs.Logger.Info("No blocks in database, initializing...")
		processInitialBlock()
		return
	}

	// Log both states to help diagnose issues
	configs.Logger.Info("Block sync status",
		zap.String("lastProcessedBlock", lastProcessedBlock),
		zap.String("latestNetworkBlock", latestBlock))

	// Check if we need to process any blocks at all
	if utils.CompareHexNumbers(latestBlock, lastProcessedBlock) <= 0 {
		configs.Logger.Info("No new blocks to process",
			zap.String("latest_db", lastProcessedBlock),
			zap.String("latest_node", latestBlock))
		return
	}

	// Use the existing GetLastSyncedBlock function to get the last synced block
	lastSyncedBlockObj, err := db.GetLastSyncedBlock()
	if err != nil {
		configs.Logger.Error("Failed to get last synced block", zap.Error(err))
	} else if lastSyncedBlockObj != nil && lastSyncedBlockObj.Result.Number != "" {
		// Compare with the current sync state
		if utils.CompareHexNumbers(lastSyncedBlockObj.Result.Number, lastProcessedBlock) > 0 {
			configs.Logger.Warn("Sync state mismatch detected - blocks exist but sync state is behind",
				zap.String("sync_state", lastProcessedBlock),
				zap.String("highest_block_found", lastSyncedBlockObj.Result.Number))

			// Force update the sync state
			forceUpdateSyncState(lastSyncedBlockObj.Result.Number)

			// Update our local variable
			lastProcessedBlock = lastSyncedBlockObj.Result.Number

			configs.Logger.Info("Sync state updated to match actual database state",
				zap.String("new_sync_state", lastProcessedBlock))
		}
	}

	// Check if we're more than BatchSyncThreshold blocks behind
	lastProcessedBlockNum := utils.HexToInt(lastProcessedBlock).Int64()
	latestBlockNum := utils.HexToInt(latestBlock).Int64()

	if latestBlockNum-lastProcessedBlockNum > BatchSyncThreshold {
		configs.Logger.Info("More than BatchSyncThreshold blocks behind, switching to batch sync",
			zap.Int64("lastProcessedBlock", lastProcessedBlockNum),
			zap.Int64("latestBlock", latestBlockNum),
			zap.Int("threshold", BatchSyncThreshold))

		// Use batch sync for faster processing
		nextBlock := utils.AddHexNumbers(lastProcessedBlock, "0x1")
		batchSync(nextBlock, latestBlock)

		// Update lastProcessedBlock to reflect what's actually been synced
		// This is important for consistent state tracking
		lastProcessedBlock = db.GetLastKnownBlockNumber()
		configs.Logger.Info("After batch sync, last synced block is now",
			zap.String("lastProcessedBlock", lastProcessedBlock))
	} else {
		// Process blocks one by one when fewer than BatchSyncThreshold blocks behind
		nextBlock := utils.AddHexNumbers(lastProcessedBlock, "0x1")
		configs.Logger.Info("Processing new blocks",
			zap.String("from", nextBlock),
			zap.String("to", latestBlock))

		// Process blocks one by one
		currentBlock := nextBlock
		failedBlocksInRun := make([]string, 0)

		for utils.CompareHexNumbers(currentBlock, latestBlock) <= 0 {
			// Check if this block has already been processed
			blockExists := db.BlockExists(currentBlock)
			if blockExists {
				configs.Logger.Info("Block already processed, skipping",
					zap.String("blockNumber", currentBlock))
				currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
				continue
			}

			configs.Logger.Info("Processing block", zap.String("blockNumber", currentBlock))

			// Process the block and check for failure
			result := processSubsequentBlocks(currentBlock)
			if result == "" {
				// Block processing failed - track for later retry
				configs.Logger.Warn("Block processing failed, will retry later",
					zap.String("blockNumber", currentBlock))
				failedBlocksInRun = append(failedBlocksInRun, currentBlock)
				currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
				continue
			}

			// Clear any previous failure tracking on success
			clearFailedBlock(currentBlock)

			processTokenTransfersForBlock(currentBlock)

			// Move to next block
			currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
		}

		// Attempt to fill any failed blocks from this run
		if len(failedBlocksInRun) > 0 {
			configs.Logger.Info("Attempting to fill failed blocks from this run",
				zap.Int("count", len(failedBlocksInRun)))
			filled := fillGaps(failedBlocksInRun)
			configs.Logger.Info("Filled failed blocks",
				zap.Int("filled", filled),
				zap.Int("remaining", len(failedBlocksInRun)-filled))
		}

		// Process all token transfers in batch after all blocks are processed
		db.ProcessTokenTransfersFromTransactions()
		configs.Logger.Info("Completed individual block processing without token transfers")

		// Update lastProcessedBlock after individual processing
		lastProcessedBlock = db.GetLastKnownBlockNumber()
		configs.Logger.Info("After individual block processing, last synced block is now",
			zap.String("lastProcessedBlock", lastProcessedBlock))
	}
}

// New function to periodically update validators
func updateValidatorsPeriodically() {
	configs.Logger.Info("Updating validators")
	err := syncValidators()
	if err != nil {
		configs.Logger.Error("Failed to update validators", zap.Error(err))
	} else {
		configs.Logger.Info("Successfully updated validators")
	}
}

func updateDataPeriodically() {
	// Update market data
	configs.Logger.Info("Updating CoinGecko data...")
	db.PeriodicallyUpdateCoinGeckoData()

	// Update wallet count
	configs.Logger.Info("Counting wallets...")
	db.CountWallets()

	// Update transaction volume
	configs.Logger.Info("Calculating daily transaction volume...")
	db.GetDailyTransactionVolume()

	// Update block size collection
	configs.Logger.Info("Updating block sizes collection...")
	if err := db.UpdateBlockSizeCollection(); err != nil {
		configs.Logger.Error("Failed to update block sizes", zap.Error(err))
	} else {
		configs.Logger.Info("Successfully updated block sizes collection")
	}
}

func singleBlockInsertion() {
	configs.Logger.Info("Starting single block insertion process")

	// Initialize collections if they don't exist
	if !db.IsCollectionsExist() {
		processInitialBlock()
	}

	// Create a wait group to keep the main goroutine alive
	var wg sync.WaitGroup
	wg.Add(4) // Added gap detection task

	// Define an initialization flag
	var initialized int32
	atomic.StoreInt32(&initialized, 0)

	// Start periodic block processing task (every 30 seconds)
	go func() {
		defer wg.Done()
		if atomic.CompareAndSwapInt32(&initialized, 0, 1) {
			configs.Logger.Info("Starting periodic task",
				zap.String("task", "block_processing"),
				zap.Duration("interval", time.Second*30))

			ticker := time.NewTicker(time.Second * 30)
			defer ticker.Stop()

			// Run immediately on start
			processBlockPeriodically()

			for range ticker.C {
				processBlockPeriodically()
			}
		}
	}()

	// Start periodic data updates task (every 30 minutes)
	go func() {
		defer wg.Done()
		configs.Logger.Info("Starting periodic task",
			zap.String("task", "data_updates"),
			zap.Duration("interval", time.Minute*30))

		ticker := time.NewTicker(time.Minute * 30)
		defer ticker.Stop()

		// Run immediately on start
		updateDataPeriodically()

		for range ticker.C {
			updateDataPeriodically()
		}
	}()

	// Start periodic validator updates task (every 6 hours)
	go func() {
		defer wg.Done()
		configs.Logger.Info("Starting periodic task",
			zap.String("task", "validator_updates"),
			zap.Duration("interval", time.Hour*6))

		ticker := time.NewTicker(time.Hour * 6)
		defer ticker.Stop()

		// Run immediately on start
		updateValidatorsPeriodically()

		for range ticker.C {
			updateValidatorsPeriodically()
		}
	}()

	// Start periodic gap detection task (every 5 minutes)
	go func() {
		defer wg.Done()
		configs.Logger.Info("Starting periodic task",
			zap.String("task", "gap_detection"),
			zap.Duration("interval", time.Minute*5))

		ticker := time.NewTicker(time.Minute * 5)
		defer ticker.Stop()

		// Wait 1 minute before first run to let initial sync settle
		time.Sleep(time.Minute)

		for range ticker.C {
			detectAndFillGapsPeriodically()
		}
	}()

	// Keep the main goroutine alive
	wg.Wait()
}

// detectAndFillGapsPeriodically runs gap detection and attempts to fill any gaps found
func detectAndFillGapsPeriodically() {
	configs.Logger.Info("Running periodic gap detection")

	// Get the current sync range
	lastKnown := db.GetLastKnownBlockNumber()
	if lastKnown == "0x0" {
		configs.Logger.Debug("No blocks synced yet, skipping gap detection")
		return
	}

	// Check the last MaxGapDetectionBlocks blocks for gaps
	lastKnownNum := utils.HexToInt(lastKnown).Int64()
	fromNum := lastKnownNum - MaxGapDetectionBlocks
	if fromNum < 1 {
		fromNum = 1
	}

	fromBlock := utils.IntToHex(int(fromNum))
	gaps := detectGaps(fromBlock, lastKnown)

	if len(gaps) == 0 {
		configs.Logger.Info("No gaps detected in block range",
			zap.String("from", fromBlock),
			zap.String("to", lastKnown))
		return
	}

	configs.Logger.Warn("Gaps detected, attempting to fill",
		zap.Int("gap_count", len(gaps)))

	filled := fillGaps(gaps)
	if filled > 0 {
		configs.Logger.Info("Periodic gap fill completed",
			zap.Int("filled", filled),
			zap.Int("remaining", len(gaps)-filled))

		// Process token transfers for filled gaps
		for _, gap := range gaps[:filled] {
			processTokenTransfersForBlock(gap)
		}
	}
}

func syncValidators() error {
	// Get current epoch from latest block
	latestBlock, err := rpc.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := strconv.FormatUint(uint64(utils.HexToInt(latestBlock).Int64()/128), 10)

	// Get validators from beacon chain
	err = rpc.GetValidators()
	if err != nil {
		configs.Logger.Error("Failed to get validators", zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully synced validators", zap.String("epoch", currentEpoch))
	return nil
}

// processTokenTransfersForBlock processes token transfers in a block
// This is called after the initial sync is complete to ensure all token transfers are captured
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
