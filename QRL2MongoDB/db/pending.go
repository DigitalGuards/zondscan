package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// UpsertPendingTransaction updates or inserts a pending transaction.
// createdAt is set only on initial insert (preserving "first seen" time across
// repeated mempool polls); lastSeen is refreshed every call.
func UpsertPendingTransaction(tx *models.PendingTransaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	tx.LastSeen = now
	tx.CreatedAt = time.Time{}

	bsonBytes, err := bson.Marshal(tx)
	if err != nil {
		configs.Logger.Error("Failed to marshal pending transaction", zap.Error(err))
		return err
	}
	var setFields bson.M
	if err := bson.Unmarshal(bsonBytes, &setFields); err != nil {
		configs.Logger.Error("Failed to unmarshal pending transaction", zap.Error(err))
		return err
	}
	// status must only be written on initial insert, otherwise a stale
	// mempool entry can revert a row that was already marked mined back to
	// "pending". Explicit transitions go through UpdatePendingTransactionStatus.
	delete(setFields, "createdAt")
	delete(setFields, "status")

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": tx.Hash}
	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			"createdAt": now,
			"status":    "pending",
		},
	}

	_, err = configs.PendingTransactionsCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to upsert pending transaction", zap.Error(err))
		return err
	}

	return nil
}

// UpdatePendingTransactionStatus updates the status of a pending transaction
func UpdatePendingTransactionStatus(hash string, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":   status,
			"lastSeen": time.Now(),
		},
	}

	_, err := configs.PendingTransactionsCollections.UpdateOne(
		ctx,
		bson.M{"_id": hash},
		update,
	)
	if err != nil {
		configs.Logger.Error("Failed to update pending transaction status", zap.Error(err))
		return err
	}

	return nil
}

// CleanupOldPendingTransactions removes rows that haven't been seen recently,
// both genuinely-pending rows the node has forgotten about, and "mined"
// tombstones that have aged out. The tombstones must be cleaned here too;
// the per-status helpers (UpdatePendingTransactionsInBlock,
// verifyPendingTransactions) intentionally leave them in place to block
// stale mempool re-inserts.
//
// Mined tombstones get the shorter minedMaxAge cutoff: once a tx is mined its
// row only exists to block a stale mempool re-poll re-inserting it as
// "pending". That race window is bounded by the mempool sync interval, so a
// mined tombstone does not need the full pending maxAge to linger. Pending
// rows keep the conservative maxAge so we never delete a row the node might
// still surface. lastSeen is set at tombstone time and never refreshed (the
// mempool no longer sees mined hashes), so it measures time-since-mined.
func CleanupOldPendingTransactions(maxAge, minedMaxAge time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	filter := bson.M{
		"$or": []bson.M{
			// Mined tombstones aged past the shorter mined cutoff.
			{
				"status":   "mined",
				"lastSeen": bson.M{"$lt": now.Add(-minedMaxAge)},
			},
			// Any non-mined (pending) row past the conservative pending cutoff.
			{
				"status":   bson.M{"$ne": "mined"},
				"lastSeen": bson.M{"$lt": now.Add(-maxAge)},
			},
		},
	}

	_, err := configs.PendingTransactionsCollections.DeleteMany(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to cleanup old pending transactions", zap.Error(err))
		return err
	}

	return nil
}

// GetAllPendingTransactionHashes returns all pending transaction hashes
func GetAllPendingTransactionHashes() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use projection to only fetch _id field for efficiency
	opts := options.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := configs.PendingTransactionsCollections.Find(ctx, bson.M{"status": "pending"}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var hashes []string
	for cursor.Next(ctx) {
		var result struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			configs.Logger.Warn("Failed to decode pending transaction hash", zap.Error(err))
			continue
		}
		hashes = append(hashes, result.ID)
	}

	return hashes, nil
}
