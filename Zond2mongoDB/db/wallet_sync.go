package db

import (
	"time"

	"Zond2mongoDB/configs"

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

	configs.Logger.Info("Wallet count sync completed",
		zap.Int64("total_wallets", count),
		zap.String("timestamp", time.Now().UTC().Format(time.RFC3339)))

	return nil
}
