// backfill-internal re-traces already-synced transactions and persists their
// nested call frames into internalTransactionByAddress. One-off maintenance
// tool for chains synced before internal-transaction indexing was enabled
// (the collection is append-only, so blocks synced with tracing disabled
// never get their internal rows otherwise).
//
// Usage: build and run next to the synchroniser's .env (replica-set MONGOURI,
// NODE_URLS, ENABLE_DEBUG_TRACE=true). The command owns the synchronizer's
// renewable chain-indexer lease for its full run. Transactions that
// already have internal rows are skipped, so re-running is safe.
package main

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/rpc"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type transferRow struct {
	TxHash         string `bson:"txHash"`
	BlockNumber    string `bson:"blockNumber"`
	BlockTimestamp string `bson:"blockTimestamp"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("internal transaction backfill interrupted: %v", err)
		} else {
			log.Printf("internal transaction backfill failed: %v", err)
		}
		_ = configs.Logger.Sync()
		os.Exit(1)
	}
	_ = configs.Logger.Sync()
}

func run(ctx context.Context) (returnErr error) {
	configs.LoadEnv()

	if strings.TrimSpace(os.Getenv("MONGOURI")) == "" {
		return fmt.Errorf("required environment variable MONGOURI is not set; maintenance writers require an explicit replica-set MongoDB URI")
	}
	if os.Getenv("ENABLE_DEBUG_TRACE") != "true" {
		return fmt.Errorf("ENABLE_DEBUG_TRACE=true is required; tracing is disabled without it")
	}
	if os.Getenv("NODE_URLS") == "" && os.Getenv("NODE_URL") == "" {
		return fmt.Errorf("required environment variable NODE_URLS or legacy NODE_URL is not set")
	}

	if err := configs.ConnectDB(); err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		returnErr = errors.Join(returnErr, configs.DB.Disconnect(disconnectCtx))
	}()

	const leaseTTL = 2 * time.Minute
	owner, err := configs.NewSyncerLeaseOwner()
	if err != nil {
		return fmt.Errorf("create maintenance lease identity: %w", err)
	}
	acquireCtx, acquireCancel := context.WithTimeout(ctx, 5*time.Second)
	lease, err := configs.AcquireSyncerLease(acquireCtx, "backfill-internal-"+owner, leaseTTL)
	acquireCancel()
	if err != nil {
		return fmt.Errorf("acquire exclusive chain-indexer maintenance lease: %w", err)
	}
	keeper, err := startMaintenanceLease(lease, leaseTTL, leaseTTL/3, configs.RenewSyncerLease)
	if err != nil {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		return errors.Join(err, configs.ReleaseSyncerLease(releaseCtx, lease))
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		returnErr = errors.Join(returnErr, keeper.CloseAndRelease(releaseCtx, configs.ReleaseSyncerLease))
	}()
	if err := configs.BootstrapDB(); err != nil {
		return fmt.Errorf("bootstrap MongoDB under maintenance lease: %w", err)
	}

	// Hashes that already have internal rows: skip them so re-runs are
	// idempotent and a partial run can resume where it stopped. Streamed
	// through a cursor rather than Distinct, which returns one BSON array
	// and hits the 16MB document cap on large collections.
	seen := make(map[string]struct{})
	seenCursor, err := configs.InternalTransactionByAddressCollections.Find(
		ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "hash", Value: 1}}))
	if err != nil {
		return fmt.Errorf("load existing internal transaction hashes: %w", err)
	}
	defer seenCursor.Close(context.Background())
	for seenCursor.Next(ctx) {
		var doc struct {
			Hash string `bson:"hash"`
		}
		if err := seenCursor.Decode(&doc); err == nil && doc.Hash != "" {
			seen[doc.Hash] = struct{}{}
		}
	}
	if err := seenCursor.Err(); err != nil {
		return fmt.Errorf("iterate existing internal transaction hashes: %w", err)
	}
	seenCursor.Close(ctx)
	log.Printf("%d transactions already have internal rows", len(seen))

	// Skip only explicit failures. The transfer collection stores an empty
	// status for a subset of transactions (the syncer's receipt-status
	// fetch is best-effort), so filtering on status == "0x1" silently
	// drops those. Anything reverted that slips through is still safe: the
	// tracer marks the root frame with an error and flattenCalls collects
	// nothing from an errored tree.
	filter := bson.D{{Key: "status", Value: bson.D{{Key: "$ne", Value: "0x0"}}}}
	opts := options.Find().SetProjection(bson.D{
		{Key: "txHash", Value: 1},
		{Key: "blockNumber", Value: 1},
		{Key: "blockTimestamp", Value: 1},
	})

	total, err := configs.TransferCollections.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("count transfer documents: %w", err)
	}
	log.Printf("scanning %d non-failed transactions", total)

	cursor, err := configs.TransferCollections.Find(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("query transfer collection: %w", err)
	}
	defer cursor.Close(context.Background())

	var scanned, skipped, traceErrs, insertErrs, txsWithCalls, rowsInserted int
	for cursor.Next(ctx) {
		if err := ctx.Err(); err != nil {
			return err
		}
		leaseCtx, leaseCancel := context.WithTimeout(ctx, 5*time.Second)
		err := keeper.EnsureHeld(leaseCtx)
		leaseCancel()
		if err != nil {
			return fmt.Errorf("maintenance lease check before transaction scan: %w", err)
		}

		var row transferRow
		if err := cursor.Decode(&row); err != nil {
			log.Printf("decode error, skipping row: %v", err)
			continue
		}
		scanned++
		if scanned%500 == 0 {
			log.Printf("progress: %d/%d scanned, %d txs with internal calls, %d rows inserted", scanned, total, txsWithCalls, rowsInserted)
		}

		if _, ok := seen[row.TxHash]; ok {
			skipped++
			continue
		}

		trace := rpc.CallDebugTraceTransaction(row.TxHash)
		if trace.Err != nil {
			traceErrs++
			log.Printf("trace failed for %s: %v", row.TxHash, trace.Err)
			continue
		}
		if len(trace.InternalCalls) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		// A failed insert may leave partial rows for this tx; the re-run
		// skip is per-hash, so check the logged hashes manually if any
		// insert errors show up in the summary.
		transactionInsertFailed := false
		for _, internalCall := range trace.InternalCalls {
			leaseCtx, leaseCancel := context.WithTimeout(ctx, 5*time.Second)
			err := keeper.EnsureHeld(leaseCtx)
			leaseCancel()
			if err != nil {
				return fmt.Errorf("maintenance lease check before internal transaction write: %w", err)
			}

			if err := db.StoreInternalCalls(
				[]rpc.InternalCall{internalCall},
				row.TxHash,
				row.BlockTimestamp,
				row.BlockNumber,
			); err != nil {
				transactionInsertFailed = true
				log.Printf("failed to store an internal call for %s: %v", row.TxHash, err)
			} else {
				rowsInserted++
			}

			leaseCtx, leaseCancel = context.WithTimeout(ctx, 5*time.Second)
			err = keeper.EnsureHeld(leaseCtx)
			leaseCancel()
			if err != nil {
				return fmt.Errorf("maintenance lease check after internal transaction write: %w", err)
			}
		}
		if transactionInsertFailed {
			insertErrs++
			continue
		}
		txsWithCalls++
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("transfer cursor after %d rows: %w", scanned, err)
	}
	if err := keeper.Err(); err != nil {
		return fmt.Errorf("chain-indexer maintenance lease failed: %w", err)
	}

	log.Printf("done: %d scanned, %d already indexed, %d trace errors, %d insert errors, %d txs produced %d internal rows",
		scanned, skipped, traceErrs, insertErrs, txsWithCalls, rowsInserted)
	return nil
}
