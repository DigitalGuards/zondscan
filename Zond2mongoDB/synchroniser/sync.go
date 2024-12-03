package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	L "Zond2mongoDB/logger"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

var logger *zap.Logger = L.FileLogger(configs.Filename)

type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

// batchSync handles syncing multiple blocks in parallel
func batchSync(fromBlock uint64, toBlock uint64) uint64 {
	// Sanity check to prevent backwards sync
	if fromBlock >= toBlock {
		logger.Error("Invalid block range for batch sync",
			zap.Uint64("from_block", fromBlock),
			zap.Uint64("to_block", toBlock))
		return fromBlock
	}

	logger.Info("Starting batch sync",
		zap.Uint64("from_block", fromBlock),
		zap.Uint64("to_block", toBlock))

	wg := sync.WaitGroup{}

	// Create buffered channel for producers
	producers := make(chan (<-chan Data), 32)

	// Start the consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(os.Stdout, producers)
	}()

	// Use larger batch size when far behind
	batchSize := 32
	if toBlock-fromBlock > 1000 {
		batchSize = 64
	}

	// Start producers in batches
	for i, size := int(fromBlock), batchSize; i < int(toBlock); i += size {
		end := i + size
		if end > int(toBlock) {
			end = int(toBlock)
		}
		producers <- producer(i, end)
		logger.Info("Processing block range",
			zap.Int("from", i),
			zap.Int("to", end))
	}

	close(producers)
	wg.Wait()

	return toBlock
}

func Sync() {
	var bNum uint64 = db.GetLatestBlockNumberFromDB() + 1
	logger.Info("Starting sync from block number", zap.Uint64("block", bNum))

	wg := sync.WaitGroup{}

	max, err := rpc.GetLatestBlock()
	if err != nil {
		logger.Error("Failed to get latest block", zap.Error(err))
		return
	}
	logger.Info("Latest block from network", zap.Uint64("block", max))

	// Create a buffered channel of read only channels, with length 32.
	producers := make(chan (<-chan Data), 32)
	logger.Info("Initialized producer channels")

	// Start the consumer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(os.Stdout, producers)
	}()
	logger.Info("Started consumer process")

	// Increased batch size for faster initial sync
	batchSize := 32
	if max-bNum > 1000 {
		batchSize = 64
	}

	// Start producers in correct order with larger batch size
	for i, size := int(bNum), batchSize; i < int(max); i += size {
		end := i + size
		if end > int(max) {
			end = int(max)
		}
		producers <- producer(i, end)
		logger.Info("Processing block range",
			zap.Int("from", i),
			zap.Int("to", end))
	}

	close(producers)
	wg.Wait()
	logger.Info("Initial sync completed successfully!")

	logger.Info("Calculating daily transaction volume...")
	db.GetDailyTransactionVolume()

	logger.Info("Starting continuous block monitoring...")
	singleBlockInsertion()
}

func consumer(w io.Writer, ch <-chan (<-chan Data)) {
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

func producer(start int, end int) <-chan Data {
	// Create a channel which we will send our data.
	Datas := make(chan Data, 32)

	var blockData []interface{}
	var blockNumbers []int

	// Start the goroutine that produces data.
	go func(ch chan<- Data) {
		defer close(ch)

		// Produce data.
		for i := start; i < end; i++ {
			data := rpc.GetBlockByNumberMainnet(uint64(i))
			if data == "" {
				logger.Warn("Empty block data received", zap.Int("block", i))
				continue
			}

			var Zond models.Zond
			err := json.Unmarshal([]byte(data), &Zond)
			if err != nil {
				logger.Error("Failed to unmarshal block data",
					zap.Int("block", i),
					zap.Error(err))
				continue
			}

			if Zond.PreResult.ParentHash != "" {
				ZondNew := db.ConvertModelsUint64(Zond)
				blockData = append(blockData, ZondNew)
				blockNumbers = append(blockNumbers, i)
			}
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
	ZondNew := rpc.GetBlockByNumberMainnet(0)
	if ZondNew == "" {
		configs.Logger.Error("Failed to get genesis block")
		return
	}

	var ZondGenesis models.ZondDatabaseBlock
	err := json.Unmarshal([]byte(ZondNew), &ZondGenesis)
	if err != nil {
		configs.Logger.Error("Failed to unmarshal genesis block", zap.Error(err))
		return
	}

	db.InsertBlockDocument(ZondGenesis)
	db.ProcessTransactions(ZondGenesis)
}

func processSubsequentBlocks(sum uint64, latestBlockNumber uint64) uint64 {
	const (
		maxRetries  = 3
		retryDelay  = 2 * time.Second
		maxRollback = uint64(50)
	)

	for retry := 0; retry < maxRetries; retry++ {
		blockData := rpc.GetBlockByNumberMainnet(sum)
		if blockData == "" {
			logger.Warn("Empty block data received",
				zap.Uint64("block", sum),
				zap.Int("retry", retry+1))
			time.Sleep(retryDelay)
			continue
		}

		var Zond models.Zond
		err := json.Unmarshal([]byte(blockData), &Zond)
		if err != nil {
			logger.Error("Failed to unmarshal block",
				zap.Uint64("block", sum),
				zap.Error(err),
				zap.Int("retry", retry+1))
			time.Sleep(retryDelay)
			continue
		}

		if Zond.PreResult.ParentHash == "" {
			logger.Warn("Block has no parent hash",
				zap.Uint64("block", sum),
				zap.Int("retry", retry+1))
			time.Sleep(retryDelay)
			continue
		}

		ZondNew := db.ConvertModelsUint64(Zond)
		previousHash := ZondNew.Result.ParentHash
		currentDBHash := db.GetLatestBlockHashHeaderFromDB(db.GetLatestBlockNumberFromDB())

		if previousHash == currentDBHash {
			processBlockAndUpdateValidators(sum, ZondNew, previousHash)
			logger.Info("Block processed successfully",
				zap.Uint64("block", sum),
				zap.String("hash", ZondNew.Result.Hash))
			return sum + 1
		}

		// If we've tried multiple times and still have hash mismatch
		if retry == maxRetries-1 {
			logger.Warn("Potential chain reorganization after retries",
				zap.Uint64("block", sum),
				zap.String("expected_parent", currentDBHash),
				zap.String("actual_parent", previousHash))

			// Walk back through the chain until we find a matching block
			var lastValidBlock uint64
			var foundFork bool
			startRollback := sum - 1

			for blockNum := startRollback; blockNum > 0 && blockNum > (startRollback-maxRollback); blockNum-- {
				prevBlockData := rpc.GetBlockByNumberMainnet(blockNum)
				if prevBlockData == "" {
					continue
				}

				var prevZond models.Zond
				if err := json.Unmarshal([]byte(prevBlockData), &prevZond); err != nil {
					continue
				}

				prevZondNew := db.ConvertModelsUint64(prevZond)
				dbHash := db.GetLatestBlockHashHeaderFromDB(blockNum)

				if dbHash == prevZondNew.Result.Hash {
					lastValidBlock = blockNum
					foundFork = true
					break
				}

				logger.Info("Rolling back block",
					zap.Uint64("block", blockNum),
					zap.String("hash", dbHash))
				db.Rollback(blockNum)
			}

			if !foundFork {
				logger.Error("Failed to find fork point within reasonable range",
					zap.Uint64("start_block", startRollback),
					zap.Uint64("end_block", startRollback-maxRollback))
				// Back off to allow network to stabilize
				time.Sleep(10 * time.Second)
				return startRollback - maxRollback
			}

			logger.Info("Found fork point, resuming sync",
				zap.Uint64("last_valid_block", lastValidBlock),
				zap.Uint64("blocks_rolled_back", sum-lastValidBlock))

			// Add a delay to allow network to stabilize
			time.Sleep(5 * time.Second)
			return lastValidBlock + 1
		}

		// If we get here, we'll retry with the same block
		logger.Info("Retrying block due to hash mismatch",
			zap.Uint64("block", sum),
			zap.Int("retry", retry+1),
			zap.String("expected_parent", currentDBHash),
			zap.String("actual_parent", previousHash))
		time.Sleep(retryDelay)
	}

	return sum
}

func processBlockAndUpdateValidators(sum uint64, ZondNew models.ZondDatabaseBlock, previousHash string) {
	db.InsertBlockDocument(ZondNew)
	db.ProcessTransactions(ZondNew)
	db.UpdateValidators(sum, previousHash)
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
	sum := db.GetLatestBlockNumberFromDB() + 1

	// Get latest block number
	latestBlockNumber, err := rpc.GetLatestBlock()
	if err != nil {
		logger.Error("Failed to get latest block", zap.Error(err))
		return
	}

	// Check if we need to process new blocks
	if latestBlockNumber > sum {
		blocksBehind := latestBlockNumber - sum
		if blocksBehind > 50 { // Use batch sync if more than 50 blocks behind
			logger.Info("Switching to batch sync mode",
				zap.Uint64("blocks_behind", blocksBehind))
			sum = batchSync(sum, latestBlockNumber)
		} else {
			sum = processSubsequentBlocks(sum, latestBlockNumber)
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
	}, time.Second*30, "data_updates")

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
