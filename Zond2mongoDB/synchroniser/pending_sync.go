package synchroniser

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const (
	MEMPOOL_SYNC_INTERVAL = 5 * time.Second
	CLEANUP_INTERVAL      = 1 * time.Hour
	MAX_PENDING_AGE       = 24 * time.Hour
)

// StartPendingTransactionSync starts the periodic mempool monitoring
func StartPendingTransactionSync() {
	// Start mempool sync
	go runPeriodicTask(func() {
		if err := syncMempool(); err != nil {
			configs.Logger.Error("Failed to sync mempool", zap.Error(err))
		}
	}, MEMPOOL_SYNC_INTERVAL, "mempool sync")

	// Start cleanup of old transactions
	go runPeriodicTask(func() {
		if err := db.CleanupOldPendingTransactions(MAX_PENDING_AGE); err != nil {
			configs.Logger.Error("Failed to cleanup old pending transactions", zap.Error(err))
		}
	}, CLEANUP_INTERVAL, "pending cleanup")
}

// UpdatePendingTransactionsInBlock checks if any pending transactions are included in the new block
func UpdatePendingTransactionsInBlock(block *models.ZondDatabaseBlock) error {
	if block == nil || len(block.Result.Transactions) == 0 {
		return nil
	}

	// Create a map of transaction hashes in the block
	blockTxs := make(map[string]bool)
	for _, tx := range block.Result.Transactions {
		blockTxs[tx.Hash] = true
	}

	// Get all pending transactions
	collection := configs.GetCollection(configs.DB, "pending_transactions")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{"status": "pending"})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return err
	}
	defer cursor.Close(ctx)

	var pendingTxs []models.PendingTransaction
	if err := cursor.All(ctx, &pendingTxs); err != nil {
		return err
	}

	// Check each pending transaction
	for _, tx := range pendingTxs {
		if blockTxs[tx.Hash] {
			// Transaction is in the block, update its status
			update := bson.M{
				"$set": bson.M{
					"status":      "mined",
					"lastSeen":    time.Now(),
					"blockNumber": block.Result.Number,
				},
			}
			_, err := collection.UpdateOne(ctx, bson.M{"_id": tx.Hash}, update)
			if err != nil {
				configs.Logger.Error("Failed to update mined transaction status",
					zap.String("hash", tx.Hash),
					zap.Error(err))
				continue
			}
			configs.Logger.Info("Transaction mined",
				zap.String("hash", tx.Hash),
				zap.String("block", block.Result.Number))
		}
	}

	return nil
}

// syncMempool fetches and processes pending transactions from the mempool
func syncMempool() error {
	// Get pending transactions from node via txpool_content
	response := rpc.GetPendingTransactions()
	if response == "" {
		configs.Logger.Debug("No response from node, txpool might be empty")
		return nil
	}

	// Parse txpool_content response format (nested by address and nonce)
	var txpoolResp models.TxPoolContentResponse
	if err := json.Unmarshal([]byte(response), &txpoolResp); err != nil {
		configs.Logger.Error("Failed to unmarshal txpool_content response",
			zap.Error(err),
			zap.String("response", response[:min(len(response), 500)]))
		return err
	}

	now := time.Now()
	count := 0

	// Process pending transactions from txpool_content format
	// txpool_content returns: {"pending": {"address": {"nonce": tx}}, "queued": {...}}
	for address, nonceTxs := range txpoolResp.Result.Pending {
		for nonce, tx := range nonceTxs {
			tx.Status = "pending"
			tx.LastSeen = now
			if tx.From == "" {
				tx.From = address
			}
			if tx.Nonce == "" {
				tx.Nonce = nonce
			}
			if err := db.UpsertPendingTransaction(&tx); err != nil {
				configs.Logger.Error("Failed to upsert pending transaction",
					zap.String("hash", tx.Hash),
					zap.Error(err))
			} else {
				count++
			}
		}
	}

	// Also process queued transactions
	for address, nonceTxs := range txpoolResp.Result.Queued {
		for nonce, tx := range nonceTxs {
			tx.Status = "pending"
			tx.LastSeen = now
			if tx.From == "" {
				tx.From = address
			}
			if tx.Nonce == "" {
				tx.Nonce = nonce
			}
			if err := db.UpsertPendingTransaction(&tx); err != nil {
				configs.Logger.Error("Failed to upsert pending transaction",
					zap.String("hash", tx.Hash),
					zap.Error(err))
			} else {
				count++
			}
		}
	}

	if count > 0 {
		configs.Logger.Info("Synced pending transactions",
			zap.Int("count", count),
			zap.Time("timestamp", now))
	} else {
		configs.Logger.Debug("No pending transactions found in txpool")
	}

	return nil
}
