package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/utils"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// GasHistoryCollection is one document per block, sized for short-range
// queries by the /gas API. We don't store all of history here, the
// `blocks` collection already has that. This collection is shaped for fast
// scans on a small (≤ 7d) timeseries.
const GasHistoryCollection = "gasHistory"

// GasHistoryRetention is how far back we keep gas-history rows. 7 d matches
// the longest range the /gas page exposes; older rows can always be
// rehydrated from `blocks` if needed.
const GasHistoryRetention = 7 * 24 * time.Hour

// gasHistoryScanLimit caps how many blocks one run pulls. It is sized to
// cover GasHistoryRetention at the network's current 60 s slot time
// (7 d ≈ 10 080 blocks) so a cold start fills the whole 7-day view in one
// pass. If slot time drops, a cold start covers proportionally less history
// and the rest fills in over subsequent runs.
const gasHistoryScanLimit = 12000

// gasHistoryRow is the bson shape written to the collection. `_id` is the
// block number (int64) so re-runs of the periodic task are idempotent.
type gasHistoryRow struct {
	ID             int64  `bson:"_id"`
	BlockNumberInt int64  `bson:"blockNumberInt"`
	Timestamp      int64  `bson:"timestamp"`
	GasUsed        string `bson:"gasUsed"`
	GasLimit       string `bson:"gasLimit"`
	BaseFeePerGas  string `bson:"baseFeePerGas"`
	TxCount        int    `bson:"txCount"`
}

// UpdateGasHistory walks forward from the highest blockNumberInt currently in
// `gasHistory` and appends one row per new block in `blocks`. It's safe to run
// repeatedly: rows are upserted on the int block number. It also sweeps rows
// older than GasHistoryRetention to keep the collection bounded.
func UpdateGasHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	col := configs.GetCollection(configs.DB, GasHistoryCollection)

	// Highwater = the largest blockNumberInt already written, or -1 if empty.
	var highwater int64 = -1
	{
		opts := options.FindOne().
			SetProjection(bson.M{"blockNumberInt": 1}).
			SetSort(bson.M{"blockNumberInt": -1})
		var existing gasHistoryRow
		err := col.FindOne(ctx, bson.M{}, opts).Decode(&existing)
		if err == nil {
			highwater = existing.BlockNumberInt
		} else if err != mongo.ErrNoDocuments {
			configs.Logger.Warn("gasHistory highwater lookup failed", zap.Error(err))
			return err
		}
	}

	// Scan newest→oldest, capped, so a run never has to hold the whole chain
	// in memory.
	//
	// Oldest-first deadlocks on a long chain. Starting from an empty
	// collection it would take the lowest block numbers, which on a chain
	// with months of history are far older than GasHistoryRetention. The
	// sweep at the end of this function then deletes every row the same run
	// just wrote, the highwater returns to -1, and the next run repeats the
	// identical batch forever, so the collection never holds anything and
	// the /gas charts stay empty. Taking the newest blocks keeps written
	// rows inside the retention window, which is what lets the highwater
	// advance.
	//
	// Block documents store these fields in lowercase BSON (gasused, gaslimit,
	// basefeepergas), both the projection paths and the local struct tags
	// below need to match that exactly.
	blockOpts := options.Find().
		SetProjection(bson.M{
			"blockNumberInt":           1,
			"result.number":            1,
			"result.timestamp":         1,
			"result.gasused":           1,
			"result.gaslimit":          1,
			"result.basefeepergas":     1,
			"result.transactions.hash": 1,
		}).
		SetSort(bson.M{"blockNumberInt": -1}).
		SetLimit(gasHistoryScanLimit)

	cur, err := configs.BlocksCollections.Find(ctx, bson.M{"blockNumberInt": bson.M{"$gt": highwater}}, blockOpts)
	if err != nil {
		configs.Logger.Error("gasHistory blocks find failed",
			zap.Int64("highwater", highwater),
			zap.Error(err))
		return err
	}
	defer cur.Close(ctx)

	configs.Logger.Info("gasHistory scan started",
		zap.Int64("highwater", highwater))

	type blockProjection struct {
		BlockNumberInt int64 `bson:"blockNumberInt"`
		Result         struct {
			Number        string `bson:"number"`
			Timestamp     string `bson:"timestamp"`
			GasUsed       string `bson:"gasused"`
			GasLimit      string `bson:"gaslimit"`
			BaseFeePerGas string `bson:"basefeepergas"`
			Transactions  []struct {
				Hash string `bson:"hash"`
			} `bson:"transactions"`
		} `bson:"result"`
	}

	var ops []mongo.WriteModel
	scanned := 0
	skippedDecode := 0
	skippedTs := 0
	for cur.Next(ctx) {
		scanned++
		var b blockProjection
		if err := cur.Decode(&b); err != nil {
			skippedDecode++
			continue
		}
		ts := utils.HexToInt64(b.Result.Timestamp)
		// Skip blocks with a malformed timestamp, they'd corrupt the
		// retention sweep below.
		if ts <= 0 {
			skippedTs++
			continue
		}
		row := gasHistoryRow{
			ID:             b.BlockNumberInt,
			BlockNumberInt: b.BlockNumberInt,
			Timestamp:      ts,
			GasUsed:        b.Result.GasUsed,
			GasLimit:       b.Result.GasLimit,
			BaseFeePerGas:  b.Result.BaseFeePerGas,
			TxCount:        len(b.Result.Transactions),
		}
		ops = append(ops, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": row.ID}).
			SetReplacement(row).
			SetUpsert(true))
	}
	if err := cur.Err(); err != nil {
		configs.Logger.Error("gasHistory cursor error",
			zap.Int("scanned", scanned),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("gasHistory scan complete",
		zap.Int64("highwater", highwater),
		zap.Int("scanned", scanned),
		zap.Int("skippedDecode", skippedDecode),
		zap.Int("skippedTs", skippedTs),
		zap.Int("ops", len(ops)))

	if len(ops) > 0 {
		_, err := col.BulkWrite(ctx, ops, options.BulkWrite().SetOrdered(false))
		if err != nil {
			configs.Logger.Error("gasHistory bulk write failed", zap.Error(err))
			return err
		}
		configs.Logger.Info("gasHistory updated", zap.Int("rows", len(ops)))
	}

	// Retention sweep
	cutoff := time.Now().Add(-GasHistoryRetention).Unix()
	if _, err := col.DeleteMany(ctx, bson.M{"timestamp": bson.M{"$lt": cutoff}}); err != nil {
		configs.Logger.Warn("gasHistory retention sweep failed", zap.Error(err))
	}

	return nil
}
