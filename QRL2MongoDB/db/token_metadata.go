// Package db / token_metadata.go
//
// Per-tokenID NFT metadata storage. Phase 3b sits on top of Phase 3a:
//
//   - The transfer dispatch loop (db/tokentransfers.go) calls StubTokenMetadata
//     on every NFT transfer with `$setOnInsert` so a row exists as soon as the
//     token is seen on chain.
//   - The metadata fetcher service polls GetTokensAwaitingMetadata, calls
//     tokenURI(id) or uri(id), resolves through the IPFS gateway, parses the
//     JSON, and writes the result via UpdateTokenMetadata.
//
// Row lifecycle (Phase 3c): a row is eligible for a fetch when it is
//
//  1. fresh: never attempted (no FetchedAt, no FetchError), or
//  2. retryable: the last attempt failed (FetchError set) and the
//     exponential-backoff deadline NextRetryAt has passed, or
//  3. stale: the last attempt succeeded (FetchedAt set, no FetchError)
//     longer than the refresh TTL ago, so an on-chain tokenURI change
//     can propagate.
//
// Same single-writer-per-concern boundary as the contract-metadata path:
// the stub writer owns URI + initial classification fields, the fetcher
// owns the resolved content fields. A failed fetch preserves last-good
// content (UpdateTokenMetadata only writes a content field when its
// argument is non-empty) and keeps the previous FetchedAt, so the explorer
// keeps serving yesterday's good data while the retry track works through
// a today-bad gateway.
package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/validation"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const tokenMetadataCollection = "tokenMetadata"

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

// StubTokenMetadata upserts a stub row for one (contract, tokenID) using
// $setOnInsert so repeated calls from the transfer dispatch loop are
// cheap no-ops. The fetcher service later fills in the resolved fields.
//
// tokenStandard is recorded in the stub so the fetcher can pick the right
// URI getter (tokenURI vs uri) without re-reading contractCode.
//
// Called from db/tokentransfers.go on every NFT transfer; must be cheap.
func StubTokenMetadata(ctx context.Context, contractAddress, tokenID, tokenStandard string) error {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	if tokenID == "" {
		return nil // nothing to key off
	}

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)
	now := time.Now().UTC().Format(time.RFC3339)

	filter := bson.M{"contractAddress": contractAddress, "tokenID": tokenID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"contractAddress": contractAddress,
			"tokenID":         tokenID,
			"tokenStandard":   tokenStandard,
			"updatedAt":       now,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		// Duplicate key on the (contract, tokenID) unique is expected on
		// repeated transfers of the same id; not an error worth logging.
		configs.Logger.Debug("Failed to upsert tokenMetadata stub",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return err
	}
	return nil
}

// UpdateTokenMetadata writes the result of a SUCCESSFUL per-token fetch.
// Empty content args do NOT clear existing values (preserve last-good);
// FetchedAt is stamped, FetchError is cleared, and the retry-scheduling
// fields are removed so the row leaves the retry track.
func UpdateTokenMetadata(
	ctx context.Context,
	contractAddress, tokenID string,
	uri, name, description, image, externalURL string,
	attributes []models.TokenAttribute,
	fetchedAt string,
) error {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	if tokenID == "" {
		return nil
	}

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)

	set := bson.M{
		"fetchedAt":  fetchedAt,
		"fetchError": "",
		"updatedAt":  time.Now().UTC().Format(time.RFC3339),
	}
	// URI is gap-fill: the stub writer doesn't know it, the fetcher
	// populates it the first time it's resolved. Re-resolution overwrites
	// because contracts can legitimately update their metadata URI scheme.
	if uri != "" {
		set["uri"] = uri
	}
	if name != "" {
		set["name"] = name
	}
	if description != "" {
		set["description"] = description
	}
	if image != "" {
		set["image"] = image
	}
	if externalURL != "" {
		set["externalURL"] = externalURL
	}
	// Attributes is treated as "all or nothing", an empty slice means
	// "preserve previous". An explicit empty list from a successful fetch
	// is uncommon enough not to optimise for.
	if len(attributes) > 0 {
		set["attributes"] = attributes
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"contractAddress": contractAddress, "tokenID": tokenID}
	update := bson.M{
		"$set":   set,
		"$unset": bson.M{"retryCount": "", "nextRetryAt": ""},
	}
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		configs.Logger.Warn("Failed to update token metadata",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return err
	}
	return nil
}

// MarkTokenMetadataFetchFailed records a failed fetch attempt: the error
// reason plus the retry-scheduling fields computed by the caller. Content
// fields and FetchedAt are untouched, so a row that succeeded in the past
// keeps serving its last-good data while it sits on the retry track.
func MarkTokenMetadataFetchFailed(
	ctx context.Context,
	contractAddress, tokenID, reason string,
	retryCount int,
	nextRetryAt string,
) error {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	if tokenID == "" {
		return nil
	}

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"contractAddress": contractAddress, "tokenID": tokenID}
	update := bson.M{"$set": bson.M{
		"fetchError":  reason,
		"retryCount":  retryCount,
		"nextRetryAt": nextRetryAt,
		"updatedAt":   time.Now().UTC().Format(time.RFC3339),
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		configs.Logger.Warn("Failed to record token metadata fetch failure",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return err
	}
	return nil
}

// missingOrEmpty matches documents where `field` is absent or the empty
// string. Mongo treats a missing field as null, and null != "", so both
// arms are needed; rows written before the retry-scheduling fields existed
// have neither.
func missingOrEmpty(field string) bson.M {
	return bson.M{"$or": []bson.M{
		{field: bson.M{"$exists": false}},
		{field: ""},
	}}
}

// dueForRetry matches errored documents whose backoff deadline has passed.
// Rows errored before the retry-scheduling fields existed have no
// `nextRetryAt` and become eligible immediately, which is exactly the
// migration behaviour we want: one deploy un-bricks every historically
// failed fetch.
func dueForRetry(errField, retryField, now string) bson.M {
	return bson.M{
		errField: bson.M{"$exists": true, "$ne": ""},
		"$or": []bson.M{
			{retryField: bson.M{"$exists": false}},
			{retryField: ""},
			{retryField: bson.M{"$lte": now}},
		},
	}
}

// findLimited runs one work-queue Find with a limit and returns the decoded
// rows. A non-positive limit short-circuits to nil.
func findLimited[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, limit int) ([]T, error) {
	if limit <= 0 {
		return nil, nil
	}
	cur, err := collection.Find(ctx, filter, options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []T
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// composeBatch assembles one work batch from the three eligibility tracks
// with guaranteed minimum shares: roughly 1/2 fresh, 1/4 retryable, 1/4
// stale (rounded in that order), with unused share flowing to the other
// tracks in fresh > retryable > stale priority. The shares are what keep
// one saturated track from starving the others: without them, a large
// population of permanently-failing rows cycling through the retry track
// would occupy every batch and silently disable the TTL-refresh track
// (and with it, on-chain URI-change propagation).
func composeBatch[T any](limit int, fresh, retryable, stale []T) []T {
	if limit <= 0 {
		return nil
	}
	freshQuota := (limit + 1) / 2
	retryQuota := (limit - freshQuota + 1) / 2
	staleQuota := limit - freshQuota - retryQuota

	out := make([]T, 0, limit)
	take := func(rows []T, n int) []T {
		if n > len(rows) {
			n = len(rows)
		}
		if n <= 0 {
			return rows
		}
		out = append(out, rows[:n]...)
		return rows[n:]
	}

	// Guaranteed shares first...
	fresh = take(fresh, freshQuota)
	retryable = take(retryable, retryQuota)
	stale = take(stale, staleQuota)
	// ...then redistribute whatever budget the short tracks left, in
	// priority order.
	fresh = take(fresh, limit-len(out))
	retryable = take(retryable, limit-len(out))
	take(stale, limit-len(out))
	return out
}

// GetTokensAwaitingMetadata returns up to `limit` token stubs that are due
// for a metadata fetch, drawn from three eligibility tracks:
//
//  1. fresh stubs that have never been attempted,
//  2. errored rows whose exponential-backoff deadline (nextRetryAt) passed,
//  3. previously successful rows whose fetchedAt is older than refreshTTL
//     (skipped when refreshTTL <= 0).
//
// The batch is composed with guaranteed per-track shares (composeBatch) so
// no track can starve the others. `now` is injected by the caller so
// eligibility is computed against a single instant per tick. Timestamps
// are RFC3339 UTC strings, which compare correctly as plain strings
// (fixed-width, most-significant-first).
func GetTokensAwaitingMetadata(ctx context.Context, limit int, now time.Time, refreshTTL time.Duration) ([]models.TokenMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)
	nowStr := now.UTC().Format(time.RFC3339)
	isNFT := bson.M{"$in": []string{"ERC-721", "ERC-1155"}}

	fresh, err := findLimited[models.TokenMetadata](ctx, collection, bson.M{
		"tokenStandard": isNFT,
		"$and": []bson.M{
			missingOrEmpty("fetchedAt"),
			missingOrEmpty("fetchError"),
		},
	}, limit)
	if err != nil {
		return nil, err
	}

	retryable, err := findLimited[models.TokenMetadata](ctx, collection, bson.M{
		"tokenStandard": isNFT,
		"$and":          []bson.M{dueForRetry("fetchError", "nextRetryAt", nowStr)},
	}, limit)
	if err != nil {
		return nil, err
	}

	var stale []models.TokenMetadata
	if refreshTTL > 0 {
		cutoff := now.UTC().Add(-refreshTTL).Format(time.RFC3339)
		stale, err = findLimited[models.TokenMetadata](ctx, collection, bson.M{
			"tokenStandard": isNFT,
			"fetchedAt":     bson.M{"$gt": "", "$lte": cutoff},
			"$and":          []bson.M{missingOrEmpty("fetchError")},
		}, limit)
		if err != nil {
			return nil, err
		}
	}

	return composeBatch(limit, fresh, retryable, stale), nil
}

// GetTokenMetadataByContract returns all per-token metadata rows for one
// collection. Used by /token/:addr/tokens enrichment.
func GetTokenMetadataByContract(ctx context.Context, contractAddress string) ([]models.TokenMetadata, error) {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)
	cur, err := collection.Find(ctx, bson.M{"contractAddress": contractAddress})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []models.TokenMetadata
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
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

// GetTokenMetadata returns the metadata document for one specific
// (contract, tokenID) pair, or nil if no stub exists.
func GetTokenMetadata(ctx context.Context, contractAddress, tokenID string) (*models.TokenMetadata, error) {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)
	var out models.TokenMetadata
	err := collection.FindOne(ctx, bson.M{"contractAddress": contractAddress, "tokenID": tokenID}).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
