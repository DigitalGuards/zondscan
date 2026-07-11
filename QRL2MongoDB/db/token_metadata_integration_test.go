//go:build integration

// Integration coverage for the metadata work-queue Mongo filters. These
// exercise real query semantics (missing vs "" fields, RFC3339 string
// comparison, $unset behaviour) that unit tests can't fake, so they need a
// live MongoDB:
//
//	mongod --dbpath /tmp/mongo-nft-test --port 28017 &
//	MONGOURI=mongodb://127.0.0.1:28017 go test -tags integration ./db/
//
// The tests use throwaway (contract, tokenID) keys inside qrldata-z and
// clean up after themselves, but do NOT point them at a production Mongo.
package db

import (
	"context"
	"testing"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"

	"go.mongodb.org/mongo-driver/bson"
)

const itContract = "Q00000000000000000000000000000000000000aa"

func cleanupTokens(t *testing.T) {
	t.Helper()
	coll := configs.GetCollection(configs.DB, tokenMetadataCollection)
	if _, err := coll.DeleteMany(context.Background(), bson.M{"contractAddress": itContract}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func tokenIDs(rows []models.TokenMetadata) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.TokenID] = true
	}
	return out
}

// TestTokenQueueTracks drives one row through the full lifecycle:
// fresh -> failed (backoff) -> due -> succeeded -> stale -> refreshed.
func TestTokenQueueTracks(t *testing.T) {
	cleanupTokens(t)
	defer cleanupTokens(t)
	ctx := context.Background()
	now := time.Now()

	// Fresh stub is eligible immediately.
	if err := StubTokenMetadata(ctx, itContract, "1", "ERC-721"); err != nil {
		t.Fatalf("stub: %v", err)
	}
	rows, err := GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if ids := tokenIDs(rows); !ids["1"] {
		t.Fatalf("fresh stub not in queue: %v", ids)
	}

	// Failure with a future deadline leaves the queue...
	future := now.Add(time.Hour).UTC().Format(time.RFC3339)
	if err := MarkTokenMetadataFetchFailed(ctx, itContract, "1", "fetch: HTTP 504", 1, future); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	rows, _ = GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if ids := tokenIDs(rows); ids["1"] {
		t.Fatal("row with future nextRetryAt must not be eligible")
	}

	// ...and re-enters once the deadline passes.
	past := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	if err := MarkTokenMetadataFetchFailed(ctx, itContract, "1", "fetch: HTTP 504", 2, past); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	rows, _ = GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if ids := tokenIDs(rows); !ids["1"] {
		t.Fatal("row with past nextRetryAt must be eligible")
	}

	// Success clears the retry track entirely.
	fetched := now.UTC().Format(time.RFC3339)
	if err := UpdateTokenMetadata(ctx, itContract, "1", "ipfs://QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "name", "", "img", "", nil, fetched); err != nil {
		t.Fatalf("success write: %v", err)
	}
	rows, _ = GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if ids := tokenIDs(rows); ids["1"] {
		t.Fatal("freshly fetched row must not be eligible")
	}
	got, err := GetTokenMetadata(ctx, itContract, "1")
	if err != nil || got == nil {
		t.Fatalf("read back: %v %v", got, err)
	}
	if got.FetchError != "" || got.RetryCount != 0 || got.NextRetryAt != "" {
		t.Fatalf("success must clear error+retry state: %+v", got)
	}

	// Older than TTL -> stale track; TTL disabled -> not eligible.
	old := now.Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	coll := configs.GetCollection(configs.DB, tokenMetadataCollection)
	if _, err := coll.UpdateOne(ctx,
		bson.M{"contractAddress": itContract, "tokenID": "1"},
		bson.M{"$set": bson.M{"fetchedAt": old}}); err != nil {
		t.Fatalf("age row: %v", err)
	}
	rows, _ = GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if ids := tokenIDs(rows); !ids["1"] {
		t.Fatal("48h-old row must be stale-eligible at 24h TTL")
	}
	rows, _ = GetTokensAwaitingMetadata(ctx, 10, now, 0)
	if ids := tokenIDs(rows); ids["1"] {
		t.Fatal("stale track must be disabled when refreshTTL <= 0")
	}
}

// TestTokenQueueLegacyRows locks in the migration behaviour for documents
// written before the retry-scheduling fields existed.
func TestTokenQueueLegacyRows(t *testing.T) {
	cleanupTokens(t)
	defer cleanupTokens(t)
	ctx := context.Background()
	now := time.Now()
	coll := configs.GetCollection(configs.DB, tokenMetadataCollection)

	// Old-style failure: fetchedAt:"" (the old writer always wrote it),
	// fetchError set, no retryCount/nextRetryAt. Must be retryable NOW.
	if _, err := coll.InsertOne(ctx, bson.M{
		"contractAddress": itContract, "tokenID": "2",
		"tokenStandard": "ERC-721",
		"fetchedAt":     "", "fetchError": "fetch: HTTP 504",
		"updatedAt": now.UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("insert legacy failure: %v", err)
	}
	// Old-style success from long ago: must be stale-eligible.
	if _, err := coll.InsertOne(ctx, bson.M{
		"contractAddress": itContract, "tokenID": "3",
		"tokenStandard": "ERC-1155",
		"fetchedAt":     now.Add(-40 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"fetchError":    "",
		"updatedAt":     now.UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("insert legacy success: %v", err)
	}
	// Non-NFT row: never eligible.
	if _, err := coll.InsertOne(ctx, bson.M{
		"contractAddress": itContract, "tokenID": "4",
		"tokenStandard": "ERC-20",
	}); err != nil {
		t.Fatalf("insert non-nft: %v", err)
	}

	rows, err := GetTokensAwaitingMetadata(ctx, 10, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	ids := tokenIDs(rows)
	if !ids["2"] {
		t.Fatal("legacy errored row (no nextRetryAt) must be immediately retryable")
	}
	if !ids["3"] {
		t.Fatal("legacy successful row past TTL must be stale-eligible")
	}
	if ids["4"] {
		t.Fatal("non-NFT rows must never be eligible")
	}
}

// TestTokenQueuePriority verifies fresh rows win the batch budget over
// retryable and stale rows when the limit is tight.
func TestTokenQueuePriority(t *testing.T) {
	cleanupTokens(t)
	defer cleanupTokens(t)
	ctx := context.Background()
	now := time.Now()

	if err := StubTokenMetadata(ctx, itContract, "10", "ERC-721"); err != nil {
		t.Fatalf("stub: %v", err)
	}
	if err := StubTokenMetadata(ctx, itContract, "11", "ERC-721"); err != nil {
		t.Fatalf("stub: %v", err)
	}
	past := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	if err := MarkTokenMetadataFetchFailed(ctx, itContract, "11", "boom", 1, past); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	rows, err := GetTokensAwaitingMetadata(ctx, 1, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(rows) != 1 || rows[0].TokenID != "10" {
		t.Fatalf("limit=1 must return the fresh row first, got %+v", rows)
	}
}
