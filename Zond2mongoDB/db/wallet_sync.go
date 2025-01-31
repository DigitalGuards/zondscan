package db

import (
	"context"
	"time"

	"Zond2mongoDB/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StartWalletCountSync starts a goroutine that syncs wallet count every 4 hours
func StartWalletCountSync() {
	configs.Logger.Info("Initializing wallet count sync service")
	go func() {
		ticker := time.NewTicker(4 * time.Hour)
		defer ticker.Stop()

		// Do an initial count immediately
		configs.Logger.Info("Performing initial wallet count sync")
		if err := syncWalletCount(); err != nil {
			configs.Logger.Error("Failed initial wallet count sync", zap.Error(err))
		}

		// Then sync every 4 hours
		configs.Logger.Info("Starting periodic wallet count sync (every 4 hours)")
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
