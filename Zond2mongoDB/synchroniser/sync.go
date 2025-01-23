package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
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
	validators := rpc.GetValidators()
	if validators.ValidatorsBySlotNumber != nil && len(validators.ValidatorsBySlotNumber) > 0 {
		db.InsertValidators(validators)
		configs.Logger.Info("Successfully initialized validators",
			zap.Int("num_slots", len(validators.ValidatorsBySlotNumber)))
	} else {
		configs.Logger.Error("Failed to initialize validators - got empty data")
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

func processSubsequentBlocks(currentBlock string) string {
	const (
		maxRetries  = 3
		retryDelay  = 2 * time.Second
		maxRollback = "0x32" // 0x32 = 50
	)

	for retry := 0; retry < maxRetries; retry++ {
		blockData, err := rpc.GetBlockByNumberMainnet(currentBlock)
		if err != nil {
			logger.Warn("Failed to get block data",
				zap.String("block", currentBlock),
				zap.Int("retry", retry+1),
				zap.Error(err))
			time.Sleep(retryDelay)
			continue
		}

		if blockData == nil || blockData.Result.ParentHash == "" {
			logger.Warn("Block has no parent hash",
				zap.String("block", currentBlock),
				zap.Int("retry", retry+1))
			time.Sleep(retryDelay)
			continue
		}

		db.UpdateTransactionStatuses(blockData)
		previousHash := blockData.Result.ParentHash
		currentDBHash := db.GetLatestBlockHashHeaderFromDB(db.GetLatestBlockNumberFromDB())

		if previousHash == currentDBHash {
			processBlockAndUpdateValidators(currentBlock, blockData, previousHash)
			logger.Info("Block processed successfully",
				zap.String("block", currentBlock),
				zap.String("hash", blockData.Result.Hash))
			return utils.AddHexNumbers(currentBlock, "0x1")
		}

		// If we've tried multiple times and still have hash mismatch
		if retry == maxRetries-1 {
			logger.Warn("Potential chain reorganization after retries",
				zap.String("block", currentBlock),
				zap.String("expected_parent", currentDBHash),
				zap.String("actual_parent", previousHash))

			// Handle chain reorganization
			startRollback := utils.SubtractHexNumbers(currentBlock, "0x1")
			lastKnownGoodBlock := db.GetLastKnownBlockNumber()

			// If we can't determine the last known good block, roll back a fixed amount
			if lastKnownGoodBlock == "0x0" {
				rollbackPoint := utils.SubtractHexNumbers(startRollback, "0x32") // Roll back 50 blocks
				logger.Warn("No last known good block found, rolling back fixed amount",
					zap.String("rollback_to", rollbackPoint))
				return rollbackPoint
			}

			// Verify the last known good block is still valid
			lastGoodBlockData, err := rpc.GetBlockByNumberMainnet(lastKnownGoodBlock)
			if err != nil {
				logger.Error("Failed to get last known good block",
					zap.String("block", lastKnownGoodBlock),
					zap.Error(err))
				return utils.SubtractHexNumbers(lastKnownGoodBlock, "0x32") // Roll back 50 blocks from last known
			}

			dbHash := db.GetLatestBlockHashHeaderFromDB(lastKnownGoodBlock)
			if dbHash != lastGoodBlockData.Result.Hash {
				logger.Warn("Last known good block no longer matches chain",
					zap.String("block", lastKnownGoodBlock),
					zap.String("db_hash", dbHash),
					zap.String("chain_hash", lastGoodBlockData.Result.Hash))
				return utils.SubtractHexNumbers(lastKnownGoodBlock, "0x32") // Roll back 50 blocks from last known
			}

			// Roll back all blocks after the last known good block
			for blockNum := startRollback; utils.CompareHexNumbers(blockNum, lastKnownGoodBlock) > 0; blockNum = utils.SubtractHexNumbers(blockNum, "0x1") {
				logger.Info("Rolling back block",
					zap.String("block", blockNum))
				db.Rollback(blockNum)
			}

			logger.Info("Chain reorganization complete",
				zap.String("last_valid_block", lastKnownGoodBlock),
				zap.String("blocks_rolled_back", utils.SubtractHexNumbers(currentBlock, lastKnownGoodBlock)))

			// Add a delay to allow network to stabilize
			time.Sleep(5 * time.Second)
			return utils.AddHexNumbers(lastKnownGoodBlock, "0x1")
		}

		// If we get here, we'll retry with the same block
		logger.Info("Retrying block due to hash mismatch",
			zap.String("block", currentBlock),
			zap.Int("retry", retry+1),
			zap.String("expected_parent", currentDBHash),
			zap.String("actual_parent", previousHash))
		time.Sleep(retryDelay)
	}

	return currentBlock
}

func processBlockAndUpdateValidators(blockNumber string, block *models.ZondDatabaseBlock, previousHash string) {
	db.InsertBlockDocument(*block)
	db.ProcessTransactions(*block)
	db.UpdateValidators(blockNumber, previousHash)
	if err := processBlock(block); err != nil {
		configs.Logger.Error("Failed to process block",
			zap.String("block", blockNumber),
			zap.Error(err))
	}
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
	const maxRetries = 3
	const retryDelay = 10 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Recovered from panic in task execution",
						zap.String("task", taskName),
						zap.Any("error", r),
						zap.Int("attempt", attempt))
				}
			}()

			logger.Info("Running periodic task",
				zap.String("task", taskName),
				zap.Int("attempt", attempt))

			task()

			logger.Info("Completed periodic task",
				zap.String("task", taskName),
				zap.Int("attempt", attempt))

			// If we get here, task completed successfully
			return
		}()

		// If we get here and it's not the last attempt, retry after delay
		if attempt < maxRetries {
			logger.Warn("Retrying task after failure",
				zap.String("task", taskName),
				zap.Int("attempt", attempt),
				zap.Duration("delay", retryDelay))
			time.Sleep(retryDelay)
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
		return
	}

	// Check if we need to process new blocks
	if utils.CompareHexNumbers(latestBlockHex, nextBlock) > 0 {
		blocksBehind := utils.SubtractHexNumbers(latestBlockHex, nextBlock)
		if utils.CompareHexNumbers(blocksBehind, "0x32") > 0 { // Use batch sync if more than 50 blocks behind
			logger.Info("Switching to batch sync mode",
				zap.String("blocks_behind", blocksBehind),
				zap.String("from_block", nextBlock),
				zap.String("to_block", latestBlockHex))

			// Store current sync state before batch sync
			db.StoreLastKnownBlockNumber(nextBlock)

			// Perform batch sync
			syncedBlock := batchSync(nextBlock, latestBlockHex)

			// Verify sync progress
			if utils.CompareHexNumbers(syncedBlock, nextBlock) > 0 {
				logger.Info("Batch sync completed successfully",
					zap.String("synced_to", syncedBlock))
				nextBlock = syncedBlock
			} else {
				logger.Warn("Batch sync made no progress",
					zap.String("last_block", nextBlock))
			}
		} else {
			// Process blocks one by one for small ranges
			nextBlock = processSubsequentBlocks(nextBlock)
		}

		// Update sync state after processing
		db.StoreLastKnownBlockNumber(nextBlock)
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
	validators := rpc.GetValidators()
	if validators.ValidatorsBySlotNumber != nil && len(validators.ValidatorsBySlotNumber) > 0 {
		db.InsertValidators(validators)
		configs.Logger.Info("Successfully updated validators",
			zap.Int("num_slots", len(validators.ValidatorsBySlotNumber)))
	} else {
		configs.Logger.Error("Failed to update validators - got empty data")
	}
}

func processBlock(block *models.ZondDatabaseBlock) error {
	// Update pending transactions that are now mined
	if err := UpdatePendingTransactionsInBlock(block); err != nil {
		configs.Logger.Error("Failed to update pending transactions in block",
			zap.String("block", block.Result.Number),
			zap.Error(err))
	}

	// Continue with existing block processing...
	return nil
}
