package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"context"
	"fmt"
	"math/big"
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
)

type Data struct {
	blockData    []interface{}
	blockNumbers []int
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

	// First, detect factory-created tokens across the entire range
	configs.Logger.Info("Starting factory-created token detection")
	if err := db.DetectFactoryCreatedTokens(initialSyncStart, maxHex); err != nil {
		configs.Logger.Error("Error while detecting factory-created tokens",
			zap.Error(err))
		// Continue with regular token processing even if factory detection fails
	}

	// Process token transfers in larger batches
	tokenBatchSize := 10
	currentBlockForTokens := initialSyncStart
	totalProcessed := 0

	for utils.CompareHexNumbers(currentBlockForTokens, maxHex) < 0 {
		endBlockForTokens := utils.AddHexNumbers(currentBlockForTokens, utils.IntToHex(tokenBatchSize))
		if utils.CompareHexNumbers(endBlockForTokens, maxHex) > 0 {
			endBlockForTokens = maxHex
		}

		configs.Logger.Info("Processing token transfers batch",
			zap.String("from", currentBlockForTokens),
			zap.String("to", endBlockForTokens))

		batchProcessed := 0
		tempBlock := currentBlockForTokens
		for utils.CompareHexNumbers(tempBlock, endBlockForTokens) < 0 {
			processTokenTransfersForBlock(tempBlock)
			tempBlock = utils.AddHexNumbers(tempBlock, "0x1")
			batchProcessed++
		}

		currentBlockForTokens = endBlockForTokens
		totalProcessed += batchProcessed

		configs.Logger.Info("Completed token transfer batch",
			zap.Int("blocks_processed", batchProcessed),
			zap.Int("total_processed", totalProcessed))

		// Add a small delay between batches to prevent overwhelming the node
		time.Sleep(86 * time.Millisecond)
	}

	configs.Logger.Info("Completed token transfer processing for all synced blocks",
		zap.Int("total_blocks_processed", totalProcessed))
}

func consumer(ch <-chan (<-chan Data)) {
	var wg sync.WaitGroup
	var syncMutex sync.Mutex // Add mutex for synchronizing block updates

	// Track the highest block number processed
	var highestProcessedBlock int = 0

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

					// Store the last block number from this batch
					if len(data.blockNumbers) > 0 {
						syncMutex.Lock() // Lock before updating sync state
						lastBlock := utils.IntToHex(data.blockNumbers[len(data.blockNumbers)-1])
						db.StoreLastKnownBlockNumber(lastBlock)
						syncMutex.Unlock() // Unlock after updating
					}

					// Track the highest block number processed
					for _, blockNum := range data.blockNumbers {
						if blockNum > highestProcessedBlock {
							highestProcessedBlock = blockNum
						}
					}
				}
			}
		}(producer)
	}
	wg.Wait()

	// After all batches are processed, update the sync state with the highest block number
	if highestProcessedBlock > 0 {
		highestBlockHex := utils.IntToHex(highestProcessedBlock)
		configs.Logger.Info("Updating sync state with highest processed block after batch processing",
			zap.String("block", highestBlockHex))
		forceUpdateSyncState(highestBlockHex)
	}
}

func producer(start string, end string) <-chan Data {
	// Create a channel which we will send our data.
	Datas := make(chan Data, 32)

	var blockData []interface{}
	var blockNumbers []int

	// Start the goroutine that produces data.
	go func(ch chan<- Data) {
		defer close(ch)

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

			data, err := rpc.GetBlockByNumberMainnet(currentBlock)
			if err != nil {
				configs.Logger.Error("Failed to get block data",
					zap.String("block", currentBlock),
					zap.Error(err))
				continue
			}

			if data != nil && data.Result.ParentHash != "" {
				db.UpdateTransactionStatuses(data)
				blockData = append(blockData, *data)
				blockNumbers = append(blockNumbers, int(utils.HexToInt(currentBlock).Int64()))
			}
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
	// Get the block data from the node
	blockData, err := rpc.GetBlockByNumberMainnet(currentBlock)
	if err != nil {
		configs.Logger.Error("Failed to get block data",
			zap.String("block", currentBlock),
			zap.Error(err))
		panic(err) // Force retry
	}

	if blockData == nil || blockData.Result.ParentHash == "" {
		configs.Logger.Error("Invalid block data received",
			zap.String("block", currentBlock))
		panic(fmt.Errorf("invalid block data for block %s", currentBlock))
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

			// Process the block
			processSubsequentBlocks(currentBlock)

			processTokenTransfersForBlock(currentBlock)

			// Move to next block
			currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
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
	wg.Add(4) // Increased from 3 to 4 for the new factory token detection task

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

	// Start periodic factory token detection task (every 4 hours)
	go func() {
		defer wg.Done()
		configs.Logger.Info("Starting periodic task",
			zap.String("task", "factory_token_detection"),
			zap.Duration("interval", time.Hour*4))

		ticker := time.NewTicker(time.Hour * 4)
		defer ticker.Stop()

		// Run immediately on start
		detectFactoryTokensPeriodically()

		for range ticker.C {
			detectFactoryTokensPeriodically()
		}
	}()

	// Keep the main goroutine alive
	wg.Wait()
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

// detectFactoryTokensPeriodically periodically scans for factory-created tokens
func detectFactoryTokensPeriodically() {
	configs.Logger.Info("Starting periodic factory token detection")

	// Get the last synced block but don't use it for now
	// We're focusing on a time range from the latest block instead
	_ = db.GetLastKnownBlockNumber()

	// Get the block from 24 hours ago as our starting point
	// This allows us to catch recent factory tokens without rescanning the entire chain
	latestBlock, err := rpc.GetLatestBlock()
	if err != nil {
		configs.Logger.Error("Failed to get latest block for factory token detection",
			zap.Error(err))
		return
	}

	// Calculate block number from ~24 hours ago (assuming ~12 second block time)
	// 24 hours = 86400 seconds / 12 seconds per block = ~7200 blocks
	blockTime := 12 // seconds
	lookbackBlocks := (24 * 60 * 60) / blockTime

	// Convert to hex and subtract
	latestBlockInt := utils.HexToInt(latestBlock)
	lookbackBlocksInt := big.NewInt(int64(lookbackBlocks))
	fromBlockInt := new(big.Int).Sub(latestBlockInt, lookbackBlocksInt)

	// Don't go below 0
	if fromBlockInt.Sign() < 0 {
		fromBlockInt = big.NewInt(0)
	}

	// Convert big.Int to hex string
	fromBlock := "0x" + fromBlockInt.Text(16)

	configs.Logger.Info("Detecting factory tokens in recent blocks",
		zap.String("fromBlock", fromBlock),
		zap.String("toBlock", latestBlock))

	// Call the detection function
	if err := db.DetectFactoryCreatedTokens(fromBlock, latestBlock); err != nil {
		configs.Logger.Error("Failed to detect factory-created tokens",
			zap.Error(err))
	}
}
