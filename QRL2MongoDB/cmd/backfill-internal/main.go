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
	// idempotent and a partial run can resume where it stopped.
	seenRaw, err := configs.InternalTransactionByAddressCollections.Distinct(ctx, "hash", bson.D{})
	if err != nil {
		log.Fatalf("failed to load existing internal tx hashes: %v", err)
	}
	seen := make(map[string]struct{}, len(seenRaw))
	for _, h := range seenRaw {
		if s, ok := h.(string); ok {
			seen[s] = struct{}{}
		}
	}
	log.Printf("%d transactions already have internal rows", len(seen))

	// Only successful transactions can move value; reverted ones paid fees
	// and did nothing else.
	filter := bson.D{{Key: "status", Value: "0x1"}}
	opts := options.Find().SetProjection(bson.D{
		{Key: "txHash", Value: 1},
		{Key: "blockNumber", Value: 1},
		{Key: "blockTimestamp", Value: 1},
	})

	total, err := configs.TransferCollections.CountDocuments(ctx, filter)
	if err != nil {
		log.Fatalf("failed to count transfer docs: %v", err)
	}
	log.Printf("scanning %d successful transactions", total)

	cursor, err := configs.TransferCollections.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("failed to query transfer collection: %v", err)
	}
	defer cursor.Close(ctx)

	var scanned, skipped, traceErrs, txsWithCalls, rowsInserted int
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

		db.StoreInternalCalls(trace.InternalCalls, row.TxHash, row.BlockTimestamp, row.BlockNumber)
		txsWithCalls++
		rowsInserted += len(trace.InternalCalls)
	}
	if err := cursor.Err(); err != nil {
		log.Fatalf("cursor error after %d rows: %v", scanned, err)
	}

	log.Printf("done: %d scanned, %d already indexed, %d trace errors, %d txs produced %d internal rows",
		scanned, skipped, traceErrs, txsWithCalls, rowsInserted)
}
