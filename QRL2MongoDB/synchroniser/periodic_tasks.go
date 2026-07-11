package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/services"
	"QRL2MongoDB/utils"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// runPeriodicTask runs a task at regular intervals with panic recovery.
// The goroutine returns promptly once stopCh is closed so a graceful
// shutdown does not start new work while in-flight DB ops are draining.
func runPeriodicTask(task func(), interval time.Duration, taskName string, stopCh <-chan struct{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				configs.Logger.Error("Recovered from panic in periodic task",
					zap.String("task", taskName),
					zap.Any("error", r))
				// Restart the task after a short delay, but exit promptly if a
				// shutdown signal arrives during the wait instead of blocking
				// the whole delay on time.Sleep.
				select {
				case <-stopCh:
					return
				case <-time.After(5 * time.Second):
					runPeriodicTask(task, interval, taskName, stopCh)
				}
			}
		}()

		configs.Logger.Info("Starting periodic task",
			zap.String("task", taskName),
			zap.Duration("interval", interval))

		// Run immediately on start
		runTaskWithRetry(task, taskName)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runTaskWithRetry(task, taskName)
			case <-stopCh:
				configs.Logger.Info("Stopping periodic task on shutdown signal",
					zap.String("task", taskName))
				return
			}
		}
	}()
}

// runTaskWithRetry executes a task with retry logic on failure
func runTaskWithRetry(task func(), taskName string) {
	maxAttempts := 5
	attempt := 1

	for attempt <= maxAttempts {
		configs.Logger.Debug("Running periodic task",
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
			configs.Logger.Debug("Completed periodic task",
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

// processBlockPeriodically checks for new blocks and processes them
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

		// Process token transfers for the batch-synced blocks
		// This is critical - without this, tokens deployed in batch-synced blocks are never detected
		configs.Logger.Info("Processing token transfers for batch-synced blocks",
			zap.String("from", nextBlock),
			zap.String("to", latestBlock))
		ProcessTokensAfterInitialSync(nextBlock, latestBlock)

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

			// processSubsequentBlocks returns the next block to process. On a
			// clean success that is currentBlock+1. On a reorg or a
			// missing-parent it returns an EARLIER block (the parent), having
			// rolled back the stale data, so the loop must resume from there
			// instead of blindly advancing +1. Detect the forward-success case
			// by comparing the returned value to currentBlock+1.
			expectedNext := utils.AddHexNumbers(currentBlock, "0x1")
			if result != expectedNext {
				// Rollback / re-sync requested: jump to the returned block. Do
				// NOT run token processing or clear failure tracking for the
				// current block; it was not advanced past.
				configs.Logger.Warn("Re-sync point returned (rollback or missing parent), resuming from earlier block",
					zap.String("from_block", currentBlock),
					zap.String("resume_block", result))
				currentBlock = result
				continue
			}

			// Clear any previous failure tracking on success
			clearFailedBlock(currentBlock)

			ProcessTokenTransfersForBlock(currentBlock)

			// Move to next block (== result on the success path)
			currentBlock = result
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

// updateValidatorsPeriodically updates validator data from the beacon chain
func updateValidatorsPeriodically() {
	configs.Logger.Info("Updating validators")
	err := syncValidators()
	if err != nil {
		configs.Logger.Error("Failed to update validators", zap.Error(err))
	} else {
		configs.Logger.Info("Successfully updated validators")
	}
}

// updateDataPeriodically updates market data, wallet counts, and other statistics
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

	// Update gas-history timeseries used by /gas page and home stats.
	// Idempotent and bounded by GasHistoryRetention.
	configs.Logger.Info("Updating gas history collection...")
	if err := db.UpdateGasHistory(); err != nil {
		configs.Logger.Error("Failed to update gas history", zap.Error(err))
	}
}

// singleBlockInsertion starts continuous block monitoring with periodic tasks.
// Each periodic goroutine watches stopCh so a graceful shutdown stops them
// from starting new work; wg.Wait then returns once they have all exited,
// which lets Sync() (and therefore main.go's doneCh) complete cleanly.
func singleBlockInsertion(stopCh <-chan struct{}) {
	configs.Logger.Info("Starting single block insertion process")

	// Initialize collections if they don't exist
	if !db.IsCollectionsExist() {
		processInitialBlock()
	}

	// Create a wait group to keep the main goroutine alive
	var wg sync.WaitGroup
	wg.Add(4) // Block processing, data updates, validator updates, gap detection

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

			for {
				select {
				case <-ticker.C:
					processBlockPeriodically()
				case <-stopCh:
					configs.Logger.Info("Stopping block processing on shutdown signal")
					return
				}
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

		for {
			select {
			case <-ticker.C:
				updateDataPeriodically()
			case <-stopCh:
				configs.Logger.Info("Stopping data updates on shutdown signal")
				return
			}
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

		for {
			select {
			case <-ticker.C:
				updateValidatorsPeriodically()
			case <-stopCh:
				configs.Logger.Info("Stopping validator updates on shutdown signal")
				return
			}
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

		// Wait 1 minute before first run to let initial sync settle, but bail
		// out early if a shutdown arrives during that initial delay.
		select {
		case <-time.After(time.Minute):
		case <-stopCh:
			configs.Logger.Info("Stopping gap detection on shutdown signal")
			return
		}

		for {
			select {
			case <-ticker.C:
				detectAndFillGapsPeriodically()
			case <-stopCh:
				configs.Logger.Info("Stopping gap detection on shutdown signal")
				return
			}
		}
	}()

	// Keep the main goroutine alive until every periodic task has stopped.
	wg.Wait()
}

// syncValidators fetches and stores validator data from the beacon chain
func syncValidators() error {
	// Get current epoch from latest block
	latestBlock, err := rpc.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}
	currentEpoch := strconv.FormatUint(uint64(utils.HexToInt(latestBlock).Int64()/128), 10)

	// Fetch and store epoch info from beacon chain
	chainHead, err := rpc.GetBeaconChainHead()
	if err != nil {
		configs.Logger.Warn("Failed to get beacon chain head", zap.Error(err))
		// Don't fail - continue with validator sync
	} else {
		if err := services.StoreEpochInfo(chainHead); err != nil {
			configs.Logger.Warn("Failed to store epoch info", zap.Error(err))
		}
	}

	// Get validators from beacon chain and persist each page. Storage lives
	// here rather than in rpc so the rpc package stays transport-only.
	pages, err := rpc.GetValidators()
	if err != nil {
		configs.Logger.Error("Failed to get validators", zap.Error(err))
		return err
	}
	for _, page := range pages {
		if err := services.StoreValidators(page, currentEpoch); err != nil {
			configs.Logger.Error("Failed to store validators", zap.Error(err))
			return err
		}
	}

	// Backfill any missing epoch history records
	currentEpochInt, _ := strconv.ParseInt(currentEpoch, 10, 64)
	if err := services.BackfillValidatorHistory(currentEpochInt); err != nil {
		configs.Logger.Warn("Failed to backfill validator history", zap.Error(err))
	}

	configs.Logger.Info("Successfully synced validators", zap.String("epoch", currentEpoch))
	return nil
}
