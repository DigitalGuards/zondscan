package db

import (
	"QRL2MongoDB/configs"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// GasHistoryCollection is one document per block, sized for short-range
// queries by the /gas API. We don't store all of history here — the
// `blocks` collection already has that. This collection is shaped for fast
// scans on a small (≤ 7d) timeseries.
const GasHistoryCollection = "gasHistory"

// GasHistoryRetention is how far back we keep gas-history rows. 7 d matches
// the longest range the /gas page exposes; older rows can always be
// rehydrated from `blocks` if needed.
const GasHistoryRetention = 7 * 24 * time.Hour

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

	// Scan new blocks newest→oldest? No — oldest→newest, capped, so we don't
	// blow memory if the syncer has just caught up from a deep backfill. The
	// retention sweep below will trim any historical excess.
	// Block documents store these fields in lowercase BSON (gasused, gaslimit,
	// basefeepergas) — both the projection paths and the local struct tags
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
		SetSort(bson.M{"blockNumberInt": 1}).
		SetLimit(5000)

	cur, err := configs.BlocksCollections.Find(ctx, bson.M{"blockNumberInt": bson.M{"$gt": highwater}}, blockOpts)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

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
	for cur.Next(ctx) {
		var b blockProjection
		if err := cur.Decode(&b); err != nil {
			continue
		}
		ts := HexToInt64(b.Result.Timestamp)
		// Skip blocks with a malformed timestamp — they'd corrupt the
		// retention sweep below.
		if ts <= 0 {
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
		return err
	}

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
