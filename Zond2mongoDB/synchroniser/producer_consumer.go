package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

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

	// MaxProducerConcurrency limits concurrent block fetching goroutines
	MaxProducerConcurrency = 8
)

// producerSem is a semaphore to limit concurrent producer goroutines
var producerSem chan struct{}

// Data holds block data and numbers for batch processing
type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

// consumer processes data from multiple producer channels
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

// producer fetches blocks in a range and sends them to a channel
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

			// Add reduced delay for bulk sync operations (5-7ms instead of 50-76ms)
			time.Sleep(getRPCDelayForBulkSync())

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
