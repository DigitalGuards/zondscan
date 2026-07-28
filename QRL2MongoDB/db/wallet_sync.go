package db

import (
	"context"
	"time"

	"QRL2MongoDB/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// SyncWalletCount performs the actual wallet count sync. The periodic
// scheduler that drives it lives in synchroniser/periodic_tasks.go
// (StartWalletCountSync); this file keeps only the db work function.
func SyncWalletCount() error {
	configs.Logger.Info("Starting wallet count sync")

	// Get the current count
	count := CountWallets()
	configs.Logger.Info("Current wallet count", zap.Int64("count", count))

	// Store the count in the database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update the wallet count in the database
	_, err := configs.WalletCountCollections.UpdateOne(
		ctx,
		bson.M{"_id": "current_count"},
		bson.M{
			"$set": bson.M{
				"count":     count,
				"timestamp": time.Now().UTC(),
			},
		},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		configs.Logger.Error("Failed to update wallet count in database",
			zap.Int64("count", count),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Wallet count sync completed",
		zap.Int64("total_wallets", count),
		zap.String("timestamp", time.Now().UTC().Format(time.RFC3339)))

	return nil
}
