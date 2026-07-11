// Package db / indexes.go
//
// Central home for MongoDB index creation, legacy-index migration, and the
// startup backfill passes that accompany them. Index NAMES and key specs are
// a frozen contract: renaming an index or reordering its keys makes Mongo
// reject CreateMany with IndexOptionsConflict and aborts the whole batch
// (see the incident notes on InitializeTokenTransfersCollection below), so
// treat every SetName(...) and bson.D key order in this file as immutable.
package db

import (
	"QRL2MongoDB/configs"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// dropLegacyConflictingIndexes scans `collection`'s indexes and drops any
// whose key spec matches one of `targets`, regardless of the index name.
// Used to migrate off auto-named indexes (Mongo's default `field_1_field_1`)
// before re-creating the same key spec under a stable name. Without this
// step Mongo rejects the second create with IndexOptionsConflict and aborts
// the whole CreateMany batch.
//
// Key match is direction-aware and order-aware (the `bson.D` is a slice).
// Pass exactly the key spec you intend to recreate.
func dropLegacyConflictingIndexes(ctx context.Context, collection *mongo.Collection, targets []bson.D) {
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		configs.Logger.Warn("Could not list indexes for legacy cleanup; existing autoindexes may cause IndexOptionsConflict",
			zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var idx struct {
			Name string `bson:"name"`
			Key  bson.D `bson:"key"`
		}
		if decErr := cursor.Decode(&idx); decErr != nil {
			continue
		}
		if idx.Name == "_id_" {
			continue
		}
		for _, want := range targets {
			if !sameKeySpec(idx.Key, want) {
				continue
			}
			if _, dErr := collection.Indexes().DropOne(ctx, idx.Name); dErr != nil {
				msg := dErr.Error()
				if !strings.Contains(msg, "IndexNotFound") &&
					!strings.Contains(msg, "ns does not exist") &&
					!strings.Contains(msg, "NamespaceNotFound") {
					configs.Logger.Warn("Could not drop legacy conflicting index",
						zap.String("indexName", idx.Name),
						zap.Error(dErr))
				}
			} else {
				configs.Logger.Info("Dropped legacy conflicting index",
					zap.String("indexName", idx.Name))
			}
			break
		}
	}
}

// sameKeySpec returns true when two index key specs are identical in field
// order, name, and direction. Mongo treats `{a:1,b:1}` and `{b:1,a:1}` as
// different indexes, so order matters for collision detection.
func sameKeySpec(a, b bson.D) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key {
			return false
		}
		if a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

// InitializeTokenTransfersCollection ensures the token transfers collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist, safe to call on every restart.
func InitializeTokenTransfersCollection() error {
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing tokenTransfers collection and indexes")

	// Drop the long-gone txHash-only unique (pre-#88). DropOne returns
	// IndexNotFound on fresh deployments, acceptable.
	if _, err := collection.Indexes().DropOne(ctx, "txHash_idx"); err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "IndexNotFound") &&
			!strings.Contains(msg, "ns does not exist") &&
			!strings.Contains(msg, "NamespaceNotFound") {
			configs.Logger.Warn("Could not drop legacy txHash_idx",
				zap.Error(err))
		}
	}

	// Drop legacy auto-named indexes that collide on key spec with the
	// named ones we're about to create. configs/setup.go used to register
	// these without SetName(), so Mongo gave them auto-generated names
	// like "contractAddress_1_blockNumber_1" / "txHash_1". When the named
	// versions below are later submitted via CreateMany, Mongo rejects
	// the whole batch with IndexOptionsConflict, including the new
	// 4-tuple unique that ERC-1155 TransferBatch + ERC-721 batch mint
	// depend on. Same pattern as the tokenBalances migration further
	// down: match by key spec (not name), drop everything that conflicts
	// with what we're about to (re)create. Crucially this includes the
	// legacy "txHash_1" unique-on-txHash, which would otherwise reject
	// every Transfer event after the first per tx (E11000 duplicate-key).
	// See https://github.com/DigitalGuards/zondscan/pull/110 for the
	// incident write-up.
	dropLegacyConflictingIndexes(ctx, collection,
		[]bson.D{
			{{Key: "contractAddress", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "from", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "to", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "txHash", Value: 1}},
			{{Key: "blockNumberInt", Value: -1}},
		},
	)

	// Create the new 4-tuple unique index BEFORE dropping the old 3-tuple
	// version. Order matters: if creation fails (duplicate-key on legacy
	// data, etc.) we want to leave the old unique in place; only after
	// the new one is online do we drop the old.
	//
	// Legacy rows (pre-tokenID) lack the field, Mongo treats missing
	// fields as `null` for index uniqueness. Since the prior 3-tuple
	// unique guaranteed at most one (txHash, contract, logIndex) row,
	// the 4-tuple `(txHash, contract, logIndex, null)` is also unique
	// by extension, no migration collision on existing data.
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("contract_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("from_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("to_block_idx"),
		},
		{
			// Phase 1 unique compound: (txHash, contract, logIndex, tokenID).
			// Replaces the 3-tuple unique with the same race-condition floor
			// while keeping ERC-1155 TransferBatch rows distinct on tokenID.
			Keys: bson.D{
				{Key: "txHash", Value: 1},
				{Key: "contractAddress", Value: 1},
				{Key: "logIndex", Value: 1},
				{Key: "tokenID", Value: 1},
			},
			Options: options.Index().
				SetName("txHash_contract_logIndex_tokenID_idx").
				SetUnique(true).
				SetBackground(true),
		},
		{
			// Per-NFT history queries: list all transfers of a specific
			// tokenID in chronological order. Non-unique; serves the
			// per-NFT page (Phase 3) and the holder timeline UI.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "tokenID", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().
				SetName("contract_tokenID_block_idx").
				SetBackground(true),
		},
		{
			// Numeric block-number sort. The contract/from/to + blockNumber
			// compound indexes above still key on the hex STRING blockNumber
			// (lexicographic), so the recency sorts in GetTokenTransfersBy*
			// use this standalone numeric index instead. Descending to match
			// the SetSort(blockNumberInt: -1) on those queries.
			Keys: bson.D{
				{Key: "blockNumberInt", Value: -1},
			},
			Options: options.Index().
				SetName("blockNumberInt_desc_idx").
				SetBackground(true),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		configs.Logger.Error("Failed to create token transfer indexes",
			zap.Error(err))
		return err
	}

	// Now that the new unique is online, retire the prior 3-tuple unique
	// from #88. Missing on fresh deployments, IndexNotFound is fine.
	if _, err := collection.Indexes().DropOne(ctx, "txHash_contract_logIndex_idx"); err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "IndexNotFound") &&
			!strings.Contains(msg, "ns does not exist") &&
			!strings.Contains(msg, "NamespaceNotFound") {
			configs.Logger.Warn("Could not drop legacy txHash_contract_logIndex_idx; new 4-tuple unique still active",
				zap.Error(err))
		}
	}

	configs.Logger.Info("Successfully initialized tokenTransfers collection and indexes")
	return nil
}

// BackfillTokenTransferBlockNumberInt populates the blockNumberInt field on any
// tokenTransfers document that predates it. Rows written before the field was
// introduced were sorted by the hex STRING blockNumber, which lex-orders
// incorrectly past width boundaries (0xffff -> 0x10000). This one-time, idempotent
// pass derives the numeric value from the existing hex blockNumber so the new
// numeric sorts (and the backend) order correctly across the full history.
//
// Design:
//   - Only touches rows missing the field ({blockNumberInt: {$exists: false}}),
//     so a second invocation after completion is a pure no-op (the cursor is
//     empty).
//   - Streams a projection of just _id + blockNumber, batches UpdateOne models
//     and flushes via BulkWrite every backfillBatchSize rows, bounded memory.
//   - The outer context is the caller's; each bulk flush uses a bounded child
//     timeout so a stalled write can't hang the pass forever.
//   - Cursor errors are logged and returned (never panic).
func BackfillTokenTransferBlockNumberInt(ctx context.Context) error {
	const backfillBatchSize = 1000

	collection := configs.GetTokenTransfersCollection()

	filter := bson.M{"blockNumberInt": bson.M{"$exists": false}}
	findOpts := options.Find().SetProjection(bson.M{"_id": 1, "blockNumber": 1})

	cursor, err := collection.Find(ctx, filter, findOpts)
	if err != nil {
		configs.Logger.Error("Backfill: failed to query tokenTransfers missing blockNumberInt",
			zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	configs.Logger.Info("Backfill: starting tokenTransfers blockNumberInt backfill")

	var (
		models  []mongo.WriteModel
		updated int64
	)

	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		flushCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		res, bwErr := collection.BulkWrite(flushCtx, models, options.BulkWrite().SetOrdered(false))
		if bwErr != nil {
			configs.Logger.Error("Backfill: bulk write failed",
				zap.Int("batchSize", len(models)),
				zap.Error(bwErr))
			return bwErr
		}
		updated += res.ModifiedCount
		models = models[:0]
		configs.Logger.Info("Backfill: tokenTransfers blockNumberInt progress",
			zap.Int64("updatedSoFar", updated))
		return nil
	}

	for cursor.Next(ctx) {
		var doc struct {
			ID          interface{} `bson:"_id"`
			BlockNumber string      `bson:"blockNumber"`
		}
		if decErr := cursor.Decode(&doc); decErr != nil {
			configs.Logger.Error("Backfill: failed to decode tokenTransfers document",
				zap.Error(decErr))
			return decErr
		}

		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": doc.ID}).
			SetUpdate(bson.M{"$set": bson.M{"blockNumberInt": HexToInt64(doc.BlockNumber)}}))

		if len(models) >= backfillBatchSize {
			if flushErr := flush(); flushErr != nil {
				return flushErr
			}
		}
	}

	if curErr := cursor.Err(); curErr != nil {
		configs.Logger.Error("Backfill: cursor error while iterating tokenTransfers",
			zap.Error(curErr))
		return curErr
	}

	// Flush the final partial batch.
	if flushErr := flush(); flushErr != nil {
		return flushErr
	}

	configs.Logger.Info("Backfill: completed tokenTransfers blockNumberInt backfill",
		zap.Int64("totalUpdated", updated))
	return nil
}

// InitializeTokenMetadataCollection sets up the tokenMetadata indexes.
// Idempotent; safe to call on every startup. Mirrors the pattern used by
// InitializeTokenBalancesCollection (Phase 2).
func InitializeTokenMetadataCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)

	configs.Logger.Info("Initializing tokenMetadata collection and indexes")

	indexes := []mongo.IndexModel{
		{
			// Phase 3b primary key. Unique because each (contract, tokenID)
			// has one off-chain metadata document.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "tokenID", Value: 1},
			},
			Options: options.Index().
				SetName("contract_tokenID_idx").
				SetUnique(true).
				SetBackground(true),
		},
		{
			// Drives the fresh + stale work-queue scans. Partial-index
			// hint isn't needed at the volume we're dealing with; the
			// scan is bounded by batchSize.
			Keys: bson.D{
				{Key: "fetchedAt", Value: 1},
			},
			Options: options.Index().SetName("fetchedAt_idx").SetBackground(true),
		},
		{
			// Drives the retryable work-queue scan (fetchError set,
			// nextRetryAt due). tokenMetadata grows one row per minted
			// token, so the 30s poll must not collection-scan at steady
			// state. contractCode (one row per contract) stays unindexed
			// on purpose; its cardinality is trivially small.
			Keys: bson.D{
				{Key: "nextRetryAt", Value: 1},
			},
			Options: options.Index().SetName("nextRetryAt_idx").SetBackground(true),
		},
		{
			// Per-collection enumeration for /token/:addr/tokens enrichment.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
			},
			Options: options.Index().SetName("contract_idx").SetBackground(true),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		configs.Logger.Error("Failed to create indexes for tokenMetadata", zap.Error(err))
		return err
	}
	configs.Logger.Info("Successfully initialized tokenMetadata collection and indexes")

	// Phase 3b backfill: seed stubs for any NFT (contract, tokenID) that
	// already exists in tokenTransfers but has no stub yet. Idempotent
	// thanks to the (contract, tokenID) unique on tokenMetadata; cheap
	// for the current scale (a $group + bulk upsert). Logged but
	// non-fatal, the live dispatch path still creates stubs going
	// forward even if this pre-population step fails.
	if err := BackfillTokenMetadataStubs(context.Background()); err != nil {
		configs.Logger.Warn("tokenMetadata stub backfill failed (non-fatal)",
			zap.Error(err))
	}

	return nil
}

// BackfillTokenMetadataStubs walks `tokenTransfers` for every distinct
// (contractAddress, tokenID, tokenStandard) tuple that matches an NFT
// standard and ensures a stub exists in `tokenMetadata`. Phase 3b stub
// upserts are wired into the live transfer dispatch path, but NFTs
// that were indexed before 3b shipped have no stub, so the fetcher
// queue starts empty. Running this on every syncer startup is cheap
// (one $group + one bulk upsert) and idempotent because the
// (contract, tokenID) unique already de-dupes.
//
// Called from InitializeTokenMetadataCollection at startup; no need to
// gate on count=0 since the upsert is a no-op for existing rows.
func BackfillTokenMetadataStubs(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	transfers := configs.GetCollection(configs.DB, "tokenTransfers")
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"tokenStandard": bson.M{"$in": []string{"ERC-721", "ERC-1155"}},
			"tokenID":       bson.M{"$exists": true, "$ne": ""},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"contract": "$contractAddress",
				"tokenID":  "$tokenID",
			},
			"tokenStandard": bson.M{"$first": "$tokenStandard"},
		}}},
	}

	cur, err := transfers.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	type agg struct {
		ID struct {
			Contract string `bson:"contract"`
			TokenID  string `bson:"tokenID"`
		} `bson:"_id"`
		TokenStandard string `bson:"tokenStandard"`
	}

	created := 0
	for cur.Next(ctx) {
		var a agg
		if decErr := cur.Decode(&a); decErr != nil {
			continue
		}
		if err := StubTokenMetadata(ctx, a.ID.Contract, a.ID.TokenID, a.TokenStandard); err != nil {
			configs.Logger.Debug("Backfill stub upsert failed",
				zap.String("contract", a.ID.Contract),
				zap.String("tokenID", a.ID.TokenID),
				zap.Error(err))
			continue
		}
		created++
	}
	if cur.Err() != nil {
		return cur.Err()
	}
	configs.Logger.Info("tokenMetadata stub backfill complete",
		zap.Int("rowsConsidered", created))
	return nil
}

// InitializePendingTokenContractsCollection ensures the pending token contracts collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist, avoiding destructive DropAll.
func InitializePendingTokenContractsCollection() error {
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing pending_token_contracts collection and indexes")

	// Create indexes for pending token contracts collection.
	// CreateMany is a no-op if the index already exists, so this is safe to call on restart.
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "txHash", Value: 1},
			},
			Options: options.Index().SetName("contract_tx_idx").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "processed", Value: 1},
			},
			Options: options.Index().SetName("processed_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for pending token contracts",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized pending_token_contracts collection and indexes")
	return nil
}
