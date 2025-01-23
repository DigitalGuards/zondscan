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

	// Start producers in batches
	currentBlock := fromBlock
	for utils.CompareHexNumbers(currentBlock, toBlock) < 0 {
		endBlock := utils.AddHexNumbers(currentBlock, utils.IntToHex(batchSize))
		if utils.CompareHexNumbers(endBlock, toBlock) > 0 {
			endBlock = toBlock
		}
		producers <- producer(currentBlock, endBlock)
		logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", endBlock))
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()

	return toBlock
}

func Sync() {
	nextBlock := utils.AddHexNumbers(db.GetLatestBlockNumberFromDB(), "0x1")
	logger.Info("Starting sync from block number", zap.String("block", nextBlock))

	wg := sync.WaitGroup{}

	maxHex, err := rpc.GetLatestBlock()
	if err != nil {
		logger.Error("Failed to get latest block", zap.Error(err))
		return
	}
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

			// Walk back through the chain until we find a matching block
			var lastValidBlock string
			var foundFork bool
			startRollback := utils.SubtractHexNumbers(currentBlock, "0x1")

			for blockNum := startRollback; utils.CompareHexNumbers(blockNum, "0x0") > 0 &&
				utils.CompareHexNumbers(utils.SubtractHexNumbers(startRollback, blockNum), maxRollback) < 0; blockNum = utils.SubtractHexNumbers(blockNum, "0x1") {

				prevBlockData, err := rpc.GetBlockByNumberMainnet(blockNum)
				if err != nil {
					logger.Warn("Failed to get previous block data",
						zap.String("block", blockNum),
						zap.Error(err))
					continue
				}

				dbHash := db.GetLatestBlockHashHeaderFromDB(blockNum)
				if dbHash == prevBlockData.Result.Hash {
					lastValidBlock = blockNum
					foundFork = true
					break
				}

				logger.Info("Rolling back block",
					zap.String("block", blockNum),
					zap.String("hash", dbHash))
				db.Rollback(blockNum)
			}

			if !foundFork {
				logger.Error("Failed to find fork point within reasonable range",
					zap.String("start_block", startRollback),
					zap.String("end_block", utils.SubtractHexNumbers(startRollback, maxRollback)))
				// Back off to allow network to stabilize
				time.Sleep(10 * time.Second)
				return utils.SubtractHexNumbers(startRollback, maxRollback)
			}

			logger.Info("Found fork point, resuming sync",
				zap.String("last_valid_block", lastValidBlock),
				zap.String("blocks_rolled_back", utils.SubtractHexNumbers(currentBlock, lastValidBlock)))

			// Add a delay to allow network to stabilize
			time.Sleep(5 * time.Second)
			return utils.AddHexNumbers(lastValidBlock, "0x1")
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
	nextBlock := utils.AddHexNumbers(db.GetLatestBlockNumberFromDB(), "0x1")

	// Get latest block number
	latestBlockHex, err := rpc.GetLatestBlock()
	if err != nil {
		logger.Error("Failed to get latest block", zap.Error(err))
		return
	}

	// Check if we need to process new blocks
	if utils.CompareHexNumbers(latestBlockHex, nextBlock) > 0 {
		blocksBehind := utils.SubtractHexNumbers(latestBlockHex, nextBlock)
		if utils.CompareHexNumbers(blocksBehind, "0x32") > 0 { // Use batch sync if more than 50 blocks behind
			logger.Info("Switching to batch sync mode",
				zap.String("blocks_behind", blocksBehind))
			nextBlock = batchSync(nextBlock, latestBlockHex)
		} else {
			nextBlock = processSubsequentBlocks(nextBlock)
		}
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
