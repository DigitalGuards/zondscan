package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

var logger *zap.Logger = configs.Logger

type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

// batchSync handles syncing multiple blocks in parallel
func batchSync(fromBlock string, toBlock string) string {
	// Sanity check to prevent backwards sync
	if utils.CompareHexNumbers(fromBlock, toBlock) >= 0 {
		logger.Error("Invalid block range for batch sync",
			zap.String("from_block", fromBlock),
			zap.String("to_block", toBlock))
		return fromBlock
	}

	logger.Info("Starting batch sync",
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
			logger.Warn("Failed to create producer, retrying...",
				zap.String("from", currentBlock),
				zap.String("to", endBlock),
				zap.Int("retry", retries+1))
			time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
		}

		if producerChan != nil {
			producers <- producerChan
			logger.Info("Processing block range",
				zap.String("from", currentBlock),
				zap.String("to", endBlock))
			currentBlock = endBlock
		} else {
			logger.Error("Failed to process block range after retries",
				zap.String("from", currentBlock),
				zap.String("to", endBlock))
			// Store the last successful block
			db.StoreLastKnownBlockNumber(currentBlock)
			return currentBlock
		}
	}

	close(producers)
	wg.Wait()

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
				logger.Info("No existing blocks found, starting from genesis")
			} else {
				logger.Info("Starting from latest block in DB",
					zap.String("block", nextBlock))
			}
		} else {
			logger.Info("Continuing from last known block",
				zap.String("block", nextBlock))
		}
		nextBlock = utils.AddHexNumbers(nextBlock, "0x1")

		// Get latest block from network
		maxHex, err = rpc.GetLatestBlock()
		if err == nil {
			break
		}
		logger.Warn("Failed to get latest block, retrying...",
			zap.Error(err),
			zap.Int("retry", retries+1))
		time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
	}

	if err != nil {
		logger.Error("Failed to get latest block after retries", zap.Error(err))
		return
	}

	logger.Info("Starting sync from block number", zap.String("block", nextBlock))
	wg := sync.WaitGroup{}
	logger.Info("Latest block from network", zap.String("block", maxHex))

	// Create a buffered channel of read only channels, with length 32.
	producers := make(chan (<-chan Data), 32)
	logger.Info("Initialized producer channels")

	// Start the consumer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(producers)
	}()
	logger.Info("Started consumer process")

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
		logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", endBlock))
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()
	logger.Info("Initial sync completed successfully!")

	logger.Info("Calculating daily transaction volume...")
	db.GetDailyTransactionVolume()

	logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion()
}

func consumer(ch <-chan (<-chan Data)) {
	// Consume the producer channels.
	for Datas := range ch {
		// Consume the Datas.
		for i := range Datas {
			if i.blockData == nil || len(i.blockData) == 0 {
				continue
			}
			// Do stuff with the Datas, in order.
			db.InsertManyBlockDocuments(i.blockData)
			logger.Info("Inserted block batch",
				zap.Int("count", len(i.blockData)))

			for x := 0; x < len(i.blockNumbers); x++ {
				db.ProcessTransactions(i.blockData[x])
			}
			logger.Info("Processed transactions for blocks",
				zap.Ints("block_numbers", i.blockNumbers))

			// Store the last block number from this batch
			if len(i.blockNumbers) > 0 {
				lastBlock := utils.IntToHex(i.blockNumbers[len(i.blockNumbers)-1])
				db.StoreLastKnownBlockNumber(lastBlock)
				logger.Debug("Updated last synced block",
					zap.String("block", lastBlock))
			}
		}
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
			data, err := rpc.GetBlockByNumberMainnet(currentBlock)
			if err != nil {
				logger.Error("Failed to get block data",
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

// Helper function to find the last valid block within a range
func findLastValidBlock(currentBlock string, maxRollback string) string {
	for blockNum := currentBlock; utils.CompareHexNumbers(blockNum, utils.SubtractHexNumbers(currentBlock, maxRollback)) >= 0; blockNum = utils.SubtractHexNumbers(blockNum, "0x1") {
		dbHash := db.GetLatestBlockHashHeaderFromDB(blockNum)
		if dbHash != "" {
			chainBlock, err := rpc.GetBlockByNumberMainnet(blockNum)
			if err == nil && chainBlock != nil && dbHash == chainBlock.Result.Hash {
				return blockNum
			}
		}
	}
	return ""
}

// Helper function to find the fork point
func findForkPoint(startBlock string, endBlock string) string {
	for blockNum := startBlock; utils.CompareHexNumbers(blockNum, endBlock) >= 0; blockNum = utils.SubtractHexNumbers(blockNum, "0x1") {
		dbHash := db.GetLatestBlockHashHeaderFromDB(blockNum)
		if dbHash == "" {
			logger.Debug("Block not found in DB during rollback search",
				zap.String("block", blockNum))
			continue
		}

		chainBlock, err := rpc.GetBlockByNumberMainnet(blockNum)
		if err != nil {
			logger.Warn("Failed to get block during rollback search",
				zap.String("block", blockNum),
				zap.Error(err))
			continue
		}

		if chainBlock != nil && dbHash == chainBlock.Result.Hash {
			return blockNum
		}
	}
	return ""
}

func processSubsequentBlocks(currentBlock string) string {
	const (
		maxRetries  = 3
		retryDelay  = 2 * time.Second
		maxRollback = "0x64" // 0x64 = 100 blocks
	)

	// Get the block data from the node
	blockData, err := rpc.GetBlockByNumberMainnet(currentBlock)
	if err != nil {
		logger.Error("Failed to get block data",
			zap.String("block", currentBlock),
			zap.Error(err))
		panic(err) // Force retry
	}

	if blockData == nil || blockData.Result.ParentHash == "" {
		logger.Error("Invalid block data received",
			zap.String("block", currentBlock))
		panic(fmt.Errorf("invalid block data for block %s", currentBlock))
	}

	// Get the parent block's hash from our DB
	parentBlockNum := utils.SubtractHexNumbers(currentBlock, "0x1")
	dbParentHash := db.GetLatestBlockHashHeaderFromDB(parentBlockNum)

	// If this is not the genesis block and we don't have the parent, we need to sync the parent first
	if parentBlockNum != "0x0" && dbParentHash == "" {
		logger.Info("Missing parent block, syncing parent first",
			zap.String("current_block", currentBlock),
			zap.String("parent_block", parentBlockNum))
		return parentBlockNum
	}

	// For non-genesis blocks, verify parent hash
	if parentBlockNum != "0x0" && blockData.Result.ParentHash != dbParentHash {
		logger.Warn("Parent hash mismatch detected",
			zap.String("block", currentBlock),
			zap.String("expected_parent", dbParentHash),
			zap.String("actual_parent", blockData.Result.ParentHash))

		// Roll back one block and try again
		err = db.Rollback(currentBlock)
		if err != nil {
			logger.Error("Failed to rollback block",
				zap.String("block", currentBlock),
				zap.Error(err))
		}
		return parentBlockNum
	}

	// Process the block
	db.UpdateTransactionStatuses(blockData)
	if err := processBlock(blockData); err != nil {
		logger.Error("Failed to process block",
			zap.String("block", currentBlock),
			zap.Error(err))
		panic(err) // Force retry
	}

	logger.Info("Block processed successfully",
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
				logger.Error("Recovered from panic in periodic task",
					zap.String("task", taskName),
					zap.Any("error", r))
				// Restart the task after a short delay
				time.Sleep(5 * time.Second)
				runPeriodicTask(task, interval, taskName)
			}
		}()

		logger.Info("Starting periodic task",
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
		logger.Info("Running periodic task",
			zap.String("task", taskName),
			zap.Int("attempt", attempt))

		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Task panicked",
						zap.String("task", taskName),
						zap.Any("error", r))
				}
			}()
			task()
			// Only mark as complete if no panic occurred
			logger.Info("Completed periodic task",
				zap.String("task", taskName),
				zap.Int("attempt", attempt))
			attempt = maxAttempts + 1 // Exit loop on success
			return
		}()

		if attempt <= maxAttempts {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			logger.Warn("Retrying task after failure",
				zap.String("task", taskName),
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay))
			time.Sleep(delay)
			attempt++
		}
	}
}

func processBlockPeriodically() {
	// Get last known block from sync state
	nextBlock := db.GetLastKnownBlockNumber()
	if nextBlock == "0x0" {
		nextBlock = db.GetLatestBlockNumberFromDB()
	}
	nextBlock = utils.AddHexNumbers(nextBlock, "0x1")

	// Get latest block number with retries
	var latestBlockHex string
	var err error
	for retries := 0; retries < 3; retries++ {
		latestBlockHex, err = rpc.GetLatestBlock()
		if err == nil {
			break
		}
		logger.Warn("Failed to get latest block, retrying...",
			zap.Error(err),
			zap.Int("retry", retries+1))
		time.Sleep(time.Duration(1<<uint(retries)) * time.Second)
	}

	if err != nil {
		logger.Error("Failed to get latest block after retries", zap.Error(err))
		panic(err) // Force task retry by panicking
	}

	// Check if we need to process new blocks
	if utils.CompareHexNumbers(latestBlockHex, nextBlock) > 0 {
		blocksBehind := utils.SubtractHexNumbers(latestBlockHex, nextBlock)
		logger.Info("Processing blocks",
			zap.String("current_block", nextBlock),
			zap.String("latest_block", latestBlockHex),
			zap.String("blocks_behind", blocksBehind))

		if utils.CompareHexNumbers(blocksBehind, "0x32") > 0 { // Use batch sync if more than 50 blocks behind
			logger.Info("Switching to batch sync mode",
				zap.String("blocks_behind", blocksBehind),
				zap.String("from_block", nextBlock),
				zap.String("to_block", latestBlockHex))
			batchSync(nextBlock, latestBlockHex)
		} else {
			// Process single block
			processSubsequentBlocks(nextBlock)
		}
	} else {
		logger.Info("No new blocks to process",
			zap.String("current_block", nextBlock),
			zap.String("latest_block", latestBlockHex))
	}
}

func updateDataPeriodically() {
	// Update market data
	logger.Info("Updating CoinGecko data...")
	db.PeriodicallyUpdateCoinGeckoData()

	// Update wallet count
	logger.Info("Counting wallets...")
	db.CountWallets()

	// Update transaction volume
	logger.Info("Calculating daily transaction volume...")
	db.GetDailyTransactionVolume()
}

func singleBlockInsertion() {
	configs.Logger.Info("Starting single block insertion process")

	// Initialize collections if they don't exist
	if !db.IsCollectionsExist() {
		processInitialBlock()
	}

	// Start periodic tasks
	go runPeriodicTask(func() {
		processBlockPeriodically()
	}, time.Second*30, "block_processing")

	go runPeriodicTask(func() {
		updateDataPeriodically()
	}, time.Minute*30, "data_updates")

	// Add new periodic task for validator updates (every 6 hours)
	go runPeriodicTask(func() {
		updateValidatorsPeriodically()
	}, time.Hour*6, "validator_updates")

	// Keep the main goroutine alive
	select {}
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

func updateSyncState(blockNumber string) string {
	// First verify block exists in DB
	blockHash := db.GetLatestBlockHashHeaderFromDB(blockNumber)
	if blockHash == "" {
		logger.Error("Cannot update sync state - block not found in DB",
			zap.String("block_number", blockNumber))
		// Roll back to last known good block
		previousBlock := utils.SubtractHexNumbers(blockNumber, "0x1")
		return findLastValidBlock(previousBlock, utils.SubtractHexNumbers(previousBlock, "0x64")) // Roll back up to 100 blocks
	}

	// Verify parent hash consistency
	parentNumber := utils.SubtractHexNumbers(blockNumber, "0x1")
	parentHash := db.GetLatestBlockHashHeaderFromDB(parentNumber)
	if parentHash == "" && parentNumber != "0x0" { // Allow genesis block to have no parent
		logger.Error("Cannot update sync state - parent block missing",
			zap.String("block_number", blockNumber),
			zap.String("parent_number", parentNumber))
		return findLastValidBlock(parentNumber, utils.SubtractHexNumbers(parentNumber, "0x64"))
	}

	// Only update sync state if block and parent verification passed
	db.StoreLastKnownBlockNumber(blockNumber)
	return blockNumber
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
		logger.Error("Failed to get validators", zap.Error(err))
		return err
	}

	logger.Info("Successfully synced validators", zap.String("epoch", currentEpoch))
	return nil
}
