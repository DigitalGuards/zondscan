package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Gap detection constants
const (
	MaxGapDetectionBlocks = 1000 // Maximum blocks to check for gaps
	GapRetryAttempts      = 3    // Number of retry attempts for filling gaps
)

// FailedBlock tracks blocks that failed to sync with retry information
type FailedBlock struct {
	BlockNumber string
	Attempts    int
	LastError   error
	LastAttempt time.Time
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
