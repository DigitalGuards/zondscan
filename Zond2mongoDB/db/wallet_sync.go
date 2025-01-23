package db

import (
	"context"
	"time"

	"Zond2mongoDB/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StartWalletCountSync starts a goroutine that syncs wallet count every 24 hours
func StartWalletCountSync() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Do an initial count
		if err := syncWalletCount(); err != nil {
			configs.Logger.Error("Failed initial wallet count sync", zap.Error(err))
		}

		// Then sync every 24 hours
		for range ticker.C {
			if err := syncWalletCount(); err != nil {
				configs.Logger.Error("Failed wallet count sync", zap.Error(err))
				continue
			}
		}
	}()
}

// syncWalletCount performs the actual wallet count sync
func syncWalletCount() error {
	configs.Logger.Info("Starting wallet count sync")

	// Get the current count
	count := CountWallets()

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
