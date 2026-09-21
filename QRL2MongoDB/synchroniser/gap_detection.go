package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"context"
	"sort"
	"strings"
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

type observedGapBlock struct {
	BlockNumberInt int64 `bson:"blockNumberInt"`
	Result         struct {
		Number string `bson:"number"`
		Hash   string `bson:"hash"`
	} `bson:"result"`
	IngestionState string `bson:"ingestionState"`
}

func repairHeights(fromNum, toNum int64, rows []observedGapBlock) []string {
	type heightState struct {
		hash     string
		seen     bool
		complete bool
	}
	states := make(map[int64]heightState, len(rows))
	for _, row := range rows {
		if row.BlockNumberInt < fromNum || row.BlockNumberInt > toNum {
			continue
		}
		canonicalNumber := utils.IntToHex(int(row.BlockNumberInt))
		rowComplete := row.Result.Number == canonicalNumber && row.Result.Hash != "" &&
			row.IngestionState == db.BlockIngestionComplete
		hash := strings.ToLower(row.Result.Hash)
		state := states[row.BlockNumberInt]
		if !state.seen {
			states[row.BlockNumberInt] = heightState{
				hash:     hash,
				seen:     true,
				complete: rowComplete,
			}
			continue
		}
		state.complete = state.complete && rowComplete && state.hash == hash
		states[row.BlockNumberInt] = state
	}

	repairs := make([]string, 0)
	for height := fromNum; height <= toNum; height++ {
		state := states[height]
		if !state.seen || !state.complete {
			repairs = append(repairs, utils.IntToHex(int(height)))
		}
	}
	return repairs
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

	// Get all existing block numbers in the range.
	// Query on blockNumberInt (int64) so the $gte/$lte comparison is numeric
	// rather than lexicographic hex string ordering.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"blockNumberInt": bson.M{
			"$gte": fromNum,
			"$lte": toNum,
		},
	}

	projection := bson.M{
		"blockNumberInt": 1,
		"result.number":  1,
		"result.hash":    1,
		"ingestionState": 1,
		"_id":            0,
	}
	cursor, err := configs.BlocksCollections.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		configs.Logger.Error("Failed to query blocks for gap detection", zap.Error(err))
		return nil
	}
	defer cursor.Close(ctx)

	var observed []observedGapBlock
	if err := cursor.All(ctx, &observed); err != nil {
		configs.Logger.Error("Failed to decode blocks for gap detection", zap.Error(err))
		return nil
	}

	// Missing rows, pending markers, unknown markers, malformed identities,
	// and same-height conflicts all require replay through canonical ingestion.
	gaps := repairHeights(fromNum, toNum, observed)

	if len(gaps) > 0 {
		configs.Logger.Warn("Found block gaps",
			zap.Int("gap_count", len(gaps)),
			zap.String("from", fromBlock),
			zap.String("to", toBlock))
	}

	return gaps
}

// fillGaps attempts to sync missing or incomplete blocks and returns the exact
// durably completed prefix.
func fillGaps(gaps []string) []string {
	if len(gaps) == 0 {
		return nil
	}
	sort.Slice(gaps, func(i, j int) bool {
		return utils.CompareHexNumbers(gaps[i], gaps[j]) < 0
	})

	configs.Logger.Info("Attempting to fill block gaps",
		zap.Int("gap_count", len(gaps)))

	filled := make([]string, 0, len(gaps))
	for _, blockNum := range gaps {
		var fillErr error
		for attempt := 1; attempt <= GapRetryAttempts; attempt++ {
			time.Sleep(getRPCDelay(false))
			data, err := rpc.GetBlockByNumberMainnet(blockNum)
			if err == nil {
				err = ingestFetchedBlock(blockNum, data)
			}
			if err == nil {
				fillErr = nil
				break
			}
			fillErr = err
			trackFailedBlock(blockNum, err)
			configs.Logger.Warn("Gap ingestion attempt failed",
				zap.String("block", blockNum),
				zap.Int("attempt", attempt),
				zap.Error(err))
			if attempt < GapRetryAttempts {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
		}
		if fillErr != nil {
			configs.Logger.Error("Stopping gap fill at unresolved canonical prefix",
				zap.String("block", blockNum),
				zap.Error(fillErr))
			break
		}

		clearFailedBlock(blockNum)
		filled = append(filled, blockNum)

		configs.Logger.Info("Filled block gap",
			zap.String("block", blockNum))
	}

	configs.Logger.Info("Gap fill completed",
		zap.Int("filled", len(filled)),
		zap.Int("total_gaps", len(gaps)))

	return filled
}

// detectAndFillGapsPeriodically runs gap detection and attempts to fill any gaps found
func detectAndFillGapsPeriodically() {
	configs.Logger.Info("Running periodic gap detection")

	// Get the current sync range. Height zero is still inspected when the
	// durable cursor is zero so a pending genesis marker can recover without a
	// process restart.
	lastKnown := db.GetLastKnownBlockNumber()

	// Check the last MaxGapDetectionBlocks blocks for gaps. Clamp to 0, not 1:
	// the genesis block is a fillable gap like any other.
	lastKnownNum := utils.HexToInt(lastKnown).Int64()
	fromNum := lastKnownNum - MaxGapDetectionBlocks
	if fromNum < 0 {
		fromNum = 0
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
	if len(filled) > 0 {
		configs.Logger.Info("Periodic gap fill completed",
			zap.Int("filled", len(filled)),
			zap.Int("remaining", len(gaps)-len(filled)))

		// Process token transfers for filled gaps
		for _, gap := range filled {
			if err := ProcessTokenTransfersForBlock(gap); err != nil {
				configs.Logger.Error("Gap token block remains queued after immediate processing",
					zap.String("blockNumber", gap),
					zap.Error(err))
			}
		}
	}
}
