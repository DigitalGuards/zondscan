package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const (
	MEMPOOL_SYNC_INTERVAL   = 1 * time.Second // Reduced for faster detection
	CLEANUP_INTERVAL        = 1 * time.Hour
	VERIFY_PENDING_INTERVAL = 5 * time.Minute
	MAX_PENDING_AGE         = 24 * time.Hour
	// MAX_MINED_TOMBSTONE_AGE is the shorter cutoff for "mined" tombstone
	// rows. A mined tombstone only needs to outlive the stale-mempool re-insert
	// race (bounded by MEMPOOL_SYNC_INTERVAL); 1h is a generous margin and
	// stops mined rows from lingering for the full MAX_PENDING_AGE.
	MAX_MINED_TOMBSTONE_AGE = 1 * time.Hour
)

type pendingVerificationOperations struct {
	withMutationLock     func(func() error) error
	countCanonicalBlocks func(bson.M) (int64, error)
	markMined            func(string) (bool, error)
}

func completedCanonicalReceiptFilter(
	hash string,
	receipt *models.TransactionReceipt,
) (bson.M, error) {
	if strings.TrimSpace(hash) == "" {
		return nil, fmt.Errorf("pending transaction hash is empty")
	}
	if receipt == nil {
		return nil, fmt.Errorf("pending transaction receipt is nil")
	}
	if receipt.Result.BlockNumber == "" || receipt.Result.BlockHash == "" {
		return nil, fmt.Errorf("receipt for %s has incomplete canonical block identity", hash)
	}
	if receipt.Result.TransactionHash != "" &&
		!strings.EqualFold(receipt.Result.TransactionHash, hash) {
		return nil, fmt.Errorf("receipt transaction hash %s does not match pending hash %s",
			receipt.Result.TransactionHash, hash)
	}
	return bson.M{
		"ingestionState":           db.BlockIngestionComplete,
		"result.number":            receipt.Result.BlockNumber,
		"result.hash":              receipt.Result.BlockHash,
		"result.transactions.hash": hash,
	}, nil
}

func countCompletedCanonicalReceiptBlocks(filter bson.M) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return configs.BlocksCollections.CountDocuments(
		ctx,
		filter,
		options.Count().SetLimit(2),
	)
}

func markPendingTransactionMined(hash string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := configs.PendingTransactionsCollections.UpdateOne(
		ctx,
		bson.M{"_id": hash, "status": "pending"},
		bson.M{"$set": bson.M{
			"status":   "mined",
			"lastSeen": time.Now().UTC(),
		}},
	)
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}

func verifyPendingTransactionWithOperations(
	hash string,
	getReceipt func(string) (*models.TransactionReceipt, error),
	operations pendingVerificationOperations,
) (*models.TransactionReceipt, bool, error) {
	receipt, err := getReceipt(hash)
	if err != nil {
		return nil, false, fmt.Errorf("get receipt for pending transaction %s: %w", hash, err)
	}
	if receipt == nil || receipt.Result.Status == "" {
		return receipt, false, nil
	}
	filter, err := completedCanonicalReceiptFilter(hash, receipt)
	if err != nil {
		return receipt, false, err
	}

	markedMined := false
	err = operations.withMutationLock(func() error {
		count, countErr := operations.countCanonicalBlocks(filter)
		if countErr != nil {
			return fmt.Errorf("count canonical blocks for pending transaction %s: %w", hash, countErr)
		}
		switch count {
		case 0:
			return nil
		case 1:
			markedMined, countErr = operations.markMined(hash)
			if countErr != nil {
				return fmt.Errorf("mark pending transaction %s mined: %w", hash, countErr)
			}
			return nil
		default:
			return fmt.Errorf("receipt for pending transaction %s matched %d complete canonical blocks",
				hash, count)
		}
	})
	return receipt, markedMined, err
}

// StartPendingTransactionSync starts the periodic mempool monitoring. The
// stopCh is threaded into each periodic task so they stop accepting new work
// when a shutdown signal arrives.
func StartPendingTransactionSync(stopCh <-chan struct{}) {
	// Start mempool sync
	runPeriodicTask(func() {
		if err := syncMempool(); err != nil {
			configs.Logger.Error("Failed to sync mempool", zap.Error(err))
		}
	}, MEMPOOL_SYNC_INTERVAL, "mempool sync", stopCh)

	// Start cleanup of old transactions
	runPeriodicTask(func() {
		if err := db.CleanupOldPendingTransactions(MAX_PENDING_AGE, MAX_MINED_TOMBSTONE_AGE); err != nil {
			configs.Logger.Error("Failed to cleanup old pending transactions", zap.Error(err))
		}
	}, CLEANUP_INTERVAL, "pending cleanup", stopCh)

	// Start verification of pending transactions against node
	runPeriodicTask(func() {
		if err := verifyPendingTransactions(); err != nil {
			configs.Logger.Error("Failed to verify pending transactions", zap.Error(err))
		}
	}, VERIFY_PENDING_INTERVAL, "pending verification", stopCh)
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

	// Tombstone: keep the row with status="mined" rather than deleting it.
	// Deletion would let a subsequent mempool poll re-insert the same hash
	// via UpsertPendingTransaction's $setOnInsert path, flipping the status
	// back to "pending". Since status is excluded from $set in the upsert,
	// an existing tombstone is preserved across re-polls.
	// CleanupOldPendingTransactions sweeps these rows once they age out.
	//
	// Only status + lastSeen change here, match the shape that
	// verifyPendingTransactions/UpdatePendingTransactionStatus writes, so
	// both tombstone paths produce identical rows. blockNumber lives in
	// the blocks/transactions collections.
	var errs []error
	for _, tx := range pendingTxs {
		if blockTxs[tx.Hash] {
			if err := db.UpdatePendingTransactionStatus(tx.Hash, "mined"); err != nil {
				configs.Logger.Error("Failed to tombstone mined transaction",
					zap.String("hash", tx.Hash),
					zap.Error(err))
				errs = append(errs, fmt.Errorf("tombstone mined transaction %s: %w", tx.Hash, err))
				continue
			}
			configs.Logger.Info("Transaction mined",
				zap.String("hash", tx.Hash),
				zap.String("block", block.Result.Number))
		}
	}

	return errors.Join(errs...)
}

// processTxPoolGroup upserts all transactions from a single txpool group (pending or
// queued). It sets Status, LastSeen, From, and Nonce on each transaction before
// writing to the database, and returns the number of successfully upserted records.
func processTxPoolGroup(txGroup map[string]map[string]models.PendingTransaction, now time.Time) int {
	count := 0
	for address, nonceTxs := range txGroup {
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
	return count
}

// syncMempool fetches and processes pending transactions from the mempool
func syncMempool() error {
	// Skip mempool polling while initial block sync is in progress to avoid
	// competing with batch block fetches for RPC bandwidth.
	if !IsInitialSyncComplete() {
		configs.Logger.Debug("Skipping mempool sync, initial block sync still in progress")
		return nil
	}

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

	// Process pending transactions from txpool_content format
	// txpool_content returns: {"pending": {"address": {"nonce": tx}}, "queued": {...}}
	count := processTxPoolGroup(txpoolResp.Result.Pending, now)
	count += processTxPoolGroup(txpoolResp.Result.Queued, now)

	if count > 0 {
		configs.Logger.Info("Synced pending transactions",
			zap.Int("count", count),
			zap.Time("timestamp", now))
	} else {
		configs.Logger.Debug("No pending transactions found in txpool")
	}

	return nil
}

// verifyPendingTransactions checks pending transactions against the node
// and tombstones any that have been mined (have a receipt). Tombstone instead
// of delete to match UpdatePendingTransactionsInBlock, a delete here could
// also race with mempool re-poll re-inserting the row as "pending".
func verifyPendingTransactions() error {
	hashes, err := db.GetAllPendingTransactionHashes()
	if err != nil {
		return err
	}

	if len(hashes) == 0 {
		return nil
	}

	configs.Logger.Info("Verifying pending transactions against node",
		zap.Int("count", len(hashes)))

	operations := pendingVerificationOperations{
		withMutationLock:     WithChainMutationLock,
		countCanonicalBlocks: countCompletedCanonicalReceiptBlocks,
		markMined:            markPendingTransactionMined,
	}
	tombstonedCount := 0
	var errs []error
	for _, hash := range hashes {
		// Receipt preparation stays outside the canonical mutation lock. The
		// final status write takes the lock, then confirms the receipt still
		// identifies exactly one complete canonical block containing this tx.
		receipt, tombstoned, err := verifyPendingTransactionWithOperations(
			hash,
			rpc.GetTransactionReceipt,
			operations,
		)
		if err != nil {
			configs.Logger.Warn("Failed to verify receipt for pending tx",
				zap.String("hash", hash),
				zap.Error(err))
			errs = append(errs, err)
			continue
		}
		if tombstoned {
			tombstonedCount++
			configs.Logger.Info("Tombstoned mined transaction in pending",
				zap.String("hash", hash),
				zap.String("blockNumber", receipt.Result.BlockNumber),
				zap.String("blockHash", receipt.Result.BlockHash))
		} else if receipt != nil && receipt.Result.Status != "" {
			configs.Logger.Debug("Receipt block is no longer canonical or pending row changed",
				zap.String("hash", hash),
				zap.String("blockNumber", receipt.Result.BlockNumber),
				zap.String("blockHash", receipt.Result.BlockHash))
		}
	}

	if tombstonedCount > 0 {
		configs.Logger.Info("Pending transaction verification complete",
			zap.Int("tombstoned", tombstonedCount),
			zap.Int("total", len(hashes)))
	}

	return errors.Join(errs...)
}
