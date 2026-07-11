// backfill-internal re-traces already-synced transactions and persists their
// nested call frames into internalTransactionByAddress. One-off maintenance
// tool for chains synced before internal-transaction indexing was enabled
// (the collection is append-only, so blocks synced with tracing disabled
// never get their internal rows otherwise).
//
// Usage: build and run next to the synchroniser's .env (MONGOURI, NODE_URLS,
// ENABLE_DEBUG_TRACE=true). Transactions that already have internal rows are
// skipped, so re-running is safe.
package main

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/rpc"
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type transferRow struct {
	TxHash         string `bson:"txHash"`
	BlockNumber    string `bson:"blockNumber"`
	BlockTimestamp string `bson:"blockTimestamp"`
}

func main() {
	if os.Getenv("ENABLE_DEBUG_TRACE") != "true" {
		log.Fatal("ENABLE_DEBUG_TRACE=true is required; without it every trace call silently no-ops and the backfill would do nothing")
	}
	if os.Getenv("NODE_URLS") == "" && os.Getenv("NODE_URL") == "" {
		log.Fatal("Required environment variable NODE_URLS (or legacy NODE_URL) is not set")
	}

	ctx := context.Background()

	// Hashes that already have internal rows: skip them so re-runs are
	// idempotent and a partial run can resume where it stopped. Streamed
	// through a cursor rather than Distinct, which returns one BSON array
	// and hits the 16MB document cap on large collections.
	seen := make(map[string]struct{})
	seenCursor, err := configs.InternalTransactionByAddressCollections.Find(
		ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "hash", Value: 1}}))
	if err != nil {
		log.Fatalf("failed to load existing internal tx hashes: %v", err)
	}
	for seenCursor.Next(ctx) {
		var doc struct {
			Hash string `bson:"hash"`
		}
		if err := seenCursor.Decode(&doc); err == nil && doc.Hash != "" {
			seen[doc.Hash] = struct{}{}
		}
	}
	if err := seenCursor.Err(); err != nil {
		seenCursor.Close(ctx)
		log.Fatalf("failed to iterate existing internal tx hashes: %v", err)
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
		log.Fatalf("failed to count transfer docs: %v", err)
	}
	log.Printf("scanning %d non-failed transactions", total)

	cursor, err := configs.TransferCollections.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("failed to query transfer collection: %v", err)
	}
	defer cursor.Close(ctx)

	var scanned, skipped, traceErrs, insertErrs, txsWithCalls, rowsInserted int
	for cursor.Next(ctx) {
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

		// A failed insert may leave partial rows for this tx; the re-run
		// skip is per-hash, so check the logged hashes manually if any
		// insert errors show up in the summary.
		if err := db.StoreInternalCalls(trace.InternalCalls, row.TxHash, row.BlockTimestamp, row.BlockNumber); err != nil {
			insertErrs++
			log.Printf("failed to store internal calls for %s: %v", row.TxHash, err)
			continue
		}
		txsWithCalls++
		rowsInserted += len(trace.InternalCalls)
	}
	if err := cursor.Err(); err != nil {
		log.Fatalf("cursor error after %d rows: %v", scanned, err)
	}

	log.Printf("done: %d scanned, %d already indexed, %d trace errors, %d insert errors, %d txs produced %d internal rows",
		scanned, skipped, traceErrs, insertErrs, txsWithCalls, rowsInserted)
}
