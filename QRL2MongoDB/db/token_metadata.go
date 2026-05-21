// Package db / token_metadata.go
//
// Per-tokenID NFT metadata storage. Phase 3b sits on top of Phase 3a:
//
//   - The transfer dispatch loop (db/tokentransfers.go) calls StubTokenMetadata
//     on every NFT transfer with `$setOnInsert` so a row exists as soon as the
//     token is seen on chain.
//   - The metadata fetcher service polls GetTokensAwaitingMetadata for stubs
//     whose URI hasn't been resolved (no FetchedAt and no FetchError), calls
//     tokenURI(id) or uri(id), resolves through the IPFS gateway, parses the
//     JSON, and writes the result via UpdateTokenMetadata.
//
// Same single-writer-per-concern boundary as the contract-metadata path:
// the stub writer owns URI + initial classification fields, the fetcher
// owns the resolved content fields. A transient fetch failure preserves
// last-good content (UpdateTokenMetadata only writes a content field
// when its argument is non-empty).
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
			// Drives the fetcher's work-queue scan (rows missing FetchedAt
			// and FetchError). Partial-index hint isn't needed at the
			// volume we're dealing with; the scan is bounded by batchSize.
			Keys: bson.D{
				{Key: "fetchedAt", Value: 1},
			},
			Options: options.Index().SetName("fetchedAt_idx").SetBackground(true),
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

// UpdateTokenMetadata writes the resolved per-token metadata. Same
// "preserve last-good" semantics as UpdateContractMetadata: empty content
// args do NOT clear existing values; only FetchedAt and FetchError are
// always written.
func UpdateTokenMetadata(
	ctx context.Context,
	contractAddress, tokenID string,
	uri, name, description, image, externalURL string,
	attributes []models.TokenAttribute,
	fetchedAt, fetchError string,
) error {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	if tokenID == "" {
		return nil
	}

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)

	set := bson.M{
		"fetchedAt":  fetchedAt,
		"fetchError": fetchError,
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
	_, err := collection.UpdateOne(ctx, filter, bson.M{"$set": set})
	if err != nil {
		configs.Logger.Warn("Failed to update token metadata",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return err
	}
	return nil
}

// GetTokensAwaitingMetadata returns up to `limit` token stubs that need a
// metadata fetch (no FetchedAt and no FetchError). Same "don't auto-retry
// errors" stance as the contract path: an operator triggers re-fetch in
// Phase 4; for now an errored row stays put until the next stub-upsert
// pass overwrites the fetchError on a future transfer.
func GetTokensAwaitingMetadata(ctx context.Context, limit int) ([]models.TokenMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{
		"tokenStandard": bson.M{"$in": []string{"ERC-721", "ERC-1155"}},
		"$and": []bson.M{
			{"$or": []bson.M{
				{"fetchedAt": bson.M{"$exists": false}},
				{"fetchedAt": ""},
			}},
			{"$or": []bson.M{
				{"fetchError": bson.M{"$exists": false}},
				{"fetchError": ""},
			}},
		},
	}
	opts := options.Find().SetLimit(int64(limit))

	collection := configs.GetCollection(configs.DB, tokenMetadataCollection)
	cur, err := collection.Find(ctx, filter, opts)
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
