package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

// Constants for retries and error handling
const (
	// These are used in retry loops throughout the code
	maxRetries  = 5  // Maximum number of retries for operations
	retryDelay  = 2  // Base delay (seconds) between retries
	maxRollback = 10 // Maximum blocks to rollback on sync issues
)

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
	batchSize := 32
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(toBlock, fromBlock), "0x3e8") > 0 { // 0x3e8 = 1000
		batchSize = 64
	}

	// Start producers in batches with retry logic
	currentBlock := fromBlock
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

		if producerChan != nil {
			producers <- producerChan
			configs.Logger.Info("Processing block range",
				zap.String("from", currentBlock),
				zap.String("to", endBlock))
			currentBlock = endBlock
		} else {
			configs.Logger.Error("Failed to process block range after retries",
				zap.String("from", currentBlock),
				zap.String("to", endBlock))
			// Store the last successful block
			db.StoreLastKnownBlockNumber(currentBlock)
			return currentBlock
		}
	}

	close(producers)
	wg.Wait()

	// Now process token transfers for the entire range
	configs.Logger.Info("Processing token transfers after batch sync",
		zap.String("from_block", fromBlock),
		zap.String("to_block", toBlock))
		
	// Process a sample of blocks for token transfers
	// Processing every block could be too intensive - so we'll do every 10th block
	currentBlockForTokens := fromBlock
	for utils.CompareHexNumbers(currentBlockForTokens, toBlock) < 0 {
		processTokenTransfersForBlock(currentBlockForTokens)
		
		// Skip ahead 10 blocks
		for i := 0; i < 10 && utils.CompareHexNumbers(currentBlockForTokens, toBlock) < 0; i++ {
			currentBlockForTokens = utils.AddHexNumbers(currentBlockForTokens, "0x1")
		}
	}

	return toBlock
}

func Sync() {
	var nextBlock string
	var maxHex string
	var err error

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
	batchSize := 32
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(maxHex, nextBlock), "0x3e8") > 0 { // 0x3e8 = 1000
		batchSize = 64
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

	configs.Logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion()
}

func consumer(ch <-chan (<-chan Data)) {
	var wg sync.WaitGroup
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
						lastBlock := utils.IntToHex(data.blockNumbers[len(data.blockNumbers)-1])
						db.StoreLastKnownBlockNumber(lastBlock)
						configs.Logger.Debug("Updated last synced block",
							zap.String("block", lastBlock))
					}
				}
			}
		}(producer)
	}
	wg.Wait()
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
	configs.Logger.Info("Processing initial block")

	// Initialize validators first
	configs.Logger.Info("Initializing validators")
	err := syncValidators()
	if err != nil {
		configs.Logger.Error("Failed to initialize validators", zap.Error(err))
	} else {
		configs.Logger.Info("Successfully initialized validators")
	}

	// Process genesis block
	block, err := rpc.GetBlockByNumberMainnet("0x0")
	if err != nil {
		configs.Logger.Error("Failed to get genesis block", zap.Error(err))
		return
	}

	db.UpdateTransactionStatuses(block)
	db.InsertBlockDocument(*block)
	db.ProcessTransactions(*block)
}

// Handles database reconsiliation in case a fork was detected
func handleFork(lastBlock string) string {
	configs.Logger.Warn("Potential block chain fork detected, attempting to find valid common ancestor")
	validBlock := findLastValidBlock(lastBlock, maxRollback)
	
	if validBlock != lastBlock {
		configs.Logger.Warn("Fork detected - pruning invalid blocks",
			zap.String("from", validBlock),
			zap.String("to", lastBlock))
		
		// Rollback the database to the valid block
		err := db.Rollback(validBlock)
		if err != nil {
			configs.Logger.Error("Failed to rollback database",
				zap.String("to_block", validBlock),
				zap.Error(err))
		}
		
		return validBlock
	}
	
	configs.Logger.Error("Could not find common ancestor within rollback limit",
		zap.String("last_block", lastBlock),
		zap.Int("max_rollback", maxRollback))
	return lastBlock
}

// Find the last valid block in the blockchain to recover from a fork
func findLastValidBlock(fromBlock string, maxRollback int) string {
	// Start from fromBlock and go back until we find a valid block
	currentBlock := fromBlock
	
	for i := 0; i < maxRollback; i++ {
		// Check if current block is valid (hash matches what's in the database)
		if validateBlockHash(currentBlock) {
			configs.Logger.Info("Found valid block", zap.String("block", currentBlock))
			return currentBlock
		}
		
		// Move to previous block
		currentBlock = utils.SubtractHexNumbers(currentBlock, "0x1")
	}
	
	// If we reached maxRollback and still don't have a valid block, return initial block
	configs.Logger.Warn("Could not find valid block within rollback limit", 
		zap.String("fromBlock", fromBlock),
		zap.Int("maxRollback", maxRollback))
	return fromBlock
}

// Determine if a block hash matches what's in the database
func validateBlockHash(blockNumber string) bool {
	// Get block from node
	nodeBlock, err := rpc.GetBlockByNumberMainnet(blockNumber)
	if err != nil {
		configs.Logger.Error("Failed to get block from node", 
			zap.String("block", blockNumber),
			zap.Error(err))
		return false
	}
	
	// Get block hash from database
	dbBlock := db.GetLatestBlockFromDB()
	if dbBlock == nil {
		return false
	}
	
	// Compare hashes
	dbBlockNumber := dbBlock.Result.Number
	if dbBlockNumber != blockNumber {
		return false
	}
	
	return dbBlock.Result.Hash == nodeBlock.Result.Hash
}

// Find the point where a fork happened
func findForkPoint(startBlock string, maxRollback int) string {
	currentBlock := startBlock
	
	for i := 0; i < maxRollback; i++ {
		if validateBlockHash(currentBlock) {
			return currentBlock
		}
		currentBlock = utils.SubtractHexNumbers(currentBlock, "0x1")
	}
	
	return "0x0" // Failed to find fork point
}

func processSubsequentBlocks(currentBlock string) string {
	const (
		maxRetries  = 3
		retryDelay  = 2 * time.Second
	)

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
	db.UpdateTransactionStatuses(blockData)
	if err := processBlock(blockData); err != nil {
		configs.Logger.Error("Failed to process block",
			zap.String("block", currentBlock),
			zap.Error(err))
		panic(err) // Force retry
	}

	configs.Logger.Info("Block processed successfully",
		zap.String("block", currentBlock),
		zap.String("hash", blockData.Result.Hash))

	// Update sync state after successful processing
	db.StoreLastKnownBlockNumber(currentBlock)

	// Return next block number
	return utils.AddHexNumbers(currentBlock, "0x1")
}

func processBlock(block *models.ZondDatabaseBlock) error {
	if block == nil || block.Result.Number == "" {
		return errors.New("invalid block data")
	}

	// Verify parent hash consistency before processing
	parentNumber := utils.SubtractHexNumbers(block.Result.Number, "0x1")
	if parentNumber != "0x0" { // Skip parent check for genesis block
		parentHash := db.GetLatestBlockHashHeaderFromDB(parentNumber)
		if parentHash == "" {
			return fmt.Errorf("parent block %s not found", parentNumber)
		}
		if parentHash != block.Result.ParentHash {
			return fmt.Errorf("parent hash mismatch for block %s: expected %s, got %s",
				block.Result.Number, parentHash, block.Result.ParentHash)
		}
	}

	// Process the block
	db.InsertBlockDocument(*block)
	db.ProcessTransactions(*block)

	return nil
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

	lastProcessedBlock := db.GetLatestBlockNumberFromDB()
	if lastProcessedBlock == "0x0" {
		configs.Logger.Info("No blocks in database, initializing...")
		processInitialBlock()
		return
	}

	// Check if we're more than 50 blocks behind
	lastProcessedBlockNum := utils.HexToInt(lastProcessedBlock).Int64()
	latestBlockNum := utils.HexToInt(latestBlock).Int64()
	
	if latestBlockNum - lastProcessedBlockNum > 50 {
		configs.Logger.Info("More than 50 blocks behind, switching to batch sync",
			zap.Int64("lastProcessedBlock", lastProcessedBlockNum),
			zap.Int64("latestBlock", latestBlockNum))
		
		// Use batch sync for faster processing
		nextBlock := utils.AddHexNumbers(lastProcessedBlock, "0x1")
		batchSync(nextBlock, latestBlock)
	} else if utils.CompareHexNumbers(latestBlock, lastProcessedBlock) > 0 {
		// Process blocks one by one when fewer than 50 blocks behind
		nextBlock := utils.AddHexNumbers(lastProcessedBlock, "0x1")
		configs.Logger.Info("Processing new blocks",
			zap.String("from", nextBlock),
			zap.String("to", latestBlock))
		
		// Process blocks one by one
		currentBlock := nextBlock
		for utils.CompareHexNumbers(currentBlock, latestBlock) <= 0 {
			configs.Logger.Info("Processing block", zap.String("blockNumber", currentBlock))
			
			// Process the block
			processSubsequentBlocks(currentBlock)
			
			// Process token transfers for this block
			processTokenTransfersForBlock(currentBlock)
			
			// Move to next block
			currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
		}
	} else {
		configs.Logger.Info("No new blocks to process",
			zap.String("latest_db", lastProcessedBlock),
			zap.String("latest_node", latestBlock))
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
}

func singleBlockInsertion() {
	configs.Logger.Info("Starting single block insertion process")

	// Initialize collections if they don't exist
	if !db.IsCollectionsExist() {
		processInitialBlock()
	}

	// Create a wait group to keep the main goroutine alive
	var wg sync.WaitGroup
	wg.Add(3)

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

// processTokenTransfersForBlock processes all token transfers in a block
// This is called after the initial sync is complete to ensure all token transfers are captured
func processTokenTransfersForBlock(blockNumber string) {
	// Get block from database to get timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.number": blockNumber}
	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter).Decode(&block)
	if err != nil {
		configs.Logger.Error("Failed to get block for token transfer processing",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return
	}

	// Process token transfers
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
