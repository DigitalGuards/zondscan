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
	"fmt"
	"os"
	"testing"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"

	"go.mongodb.org/mongo-driver/bson"
)

const itContract = "Q00000000000000000000000000000000000000aa"

// TestMain connects to the MONGOURI Mongo before any integration test runs:
// configs no longer connects at import time, so tagged runs must do it here.
func TestMain(m *testing.M) {
	if err := configs.ConnectDB(); err != nil {
		fmt.Println("integration tests need a live MongoDB:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

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

// TestComposeBatch locks in the per-track share arithmetic that prevents
// one saturated track from starving the others. Pure function; lives in
// the integration file only because the db package's configs init needs
// MONGOURI at test start.
func TestComposeBatch(t *testing.T) {
	seq := func(prefix string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = prefix
		}
		return out
	}
	count := func(rows []string, prefix string) int {
		n := 0
		for _, r := range rows {
			if r == prefix {
				n++
			}
		}
		return n
	}

	// All tracks saturated at limit 25: 13/6/6.
	got := composeBatch(25, seq("f", 100), seq("r", 100), seq("s", 100))
	if len(got) != 25 || count(got, "f") != 13 || count(got, "r") != 6 || count(got, "s") != 6 {
		t.Fatalf("saturated split = %d f=%d r=%d s=%d, want 25 f=13 r=6 s=6",
			len(got), count(got, "f"), count(got, "r"), count(got, "s"))
	}

	// Retry track saturated, no fresh: leftover flows to retry BEFORE
	// stale, but stale keeps its guaranteed share (the starvation fix).
	got = composeBatch(25, nil, seq("r", 100), seq("s", 100))
	if len(got) != 25 || count(got, "r") != 19 || count(got, "s") != 6 {
		t.Fatalf("no-fresh split = %d r=%d s=%d, want 25 r=19 s=6",
			len(got), count(got, "r"), count(got, "s"))
	}

	// Only fresh rows: takes the whole budget.
	got = composeBatch(25, seq("f", 100), nil, nil)
	if len(got) != 25 || count(got, "f") != 25 {
		t.Fatalf("fresh-only = %d, want 25 fresh", len(got))
	}

	// Fewer rows than budget: everything is returned.
	got = composeBatch(25, seq("f", 2), seq("r", 3), seq("s", 1))
	if len(got) != 6 {
		t.Fatalf("under-budget = %d rows, want 6", len(got))
	}

	// Tiny limit: fresh wins by priority.
	got = composeBatch(1, seq("f", 5), seq("r", 5), seq("s", 5))
	if len(got) != 1 || got[0] != "f" {
		t.Fatalf("limit=1 = %v, want [f]", got)
	}

	// Non-positive limit: empty.
	if got = composeBatch(0, seq("f", 5), nil, nil); len(got) != 0 {
		t.Fatalf("limit=0 returned %d rows", len(got))
	}
}

// TestTokenQueueNoStaleStarvation reproduces the review finding: a
// saturated retry track must not stop stale rows from being TTL-refreshed.
func TestTokenQueueNoStaleStarvation(t *testing.T) {
	cleanupTokens(t)
	defer cleanupTokens(t)
	ctx := context.Background()
	now := time.Now()
	coll := configs.GetCollection(configs.DB, tokenMetadataCollection)

	// 30 retryable-due rows (more than the 25 batch), 1 stale row.
	past := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	for i := 0; i < 30; i++ {
		if _, err := coll.InsertOne(ctx, bson.M{
			"contractAddress": itContract, "tokenID": fmt.Sprintf("r%d", i),
			"tokenStandard": "ERC-721",
			"fetchError":    "fetch: HTTP 504",
			"retryCount":    3,
			"nextRetryAt":   past,
		}); err != nil {
			t.Fatalf("insert retryable: %v", err)
		}
	}
	if _, err := coll.InsertOne(ctx, bson.M{
		"contractAddress": itContract, "tokenID": "stale-1",
		"tokenStandard": "ERC-721",
		"fetchedAt":     now.Add(-48 * time.Hour).UTC().Format(time.RFC3339),
		"fetchError":    "",
	}); err != nil {
		t.Fatalf("insert stale: %v", err)
	}

	rows, err := GetTokensAwaitingMetadata(ctx, 25, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(rows) != 25 {
		t.Fatalf("batch size = %d, want 25", len(rows))
	}
	if ids := tokenIDs(rows); !ids["stale-1"] {
		t.Fatal("stale row starved out of a batch dominated by retryables")
	}
}
