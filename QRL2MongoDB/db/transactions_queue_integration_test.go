//go:build integration

package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPendingTokenQueueClaimAndRetentionIndexes(t *testing.T) {
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)

	type indexRow struct {
		Name               string `bson:"name"`
		Key                bson.D `bson:"key"`
		ExpireAfterSeconds int64  `bson:"expireAfterSeconds"`
	}
	indexes := make(map[string]indexRow)
	for cursor.Next(ctx) {
		var row indexRow
		if err := cursor.Decode(&row); err != nil {
			t.Fatal(err)
		}
		indexes[row.Name] = row
	}
	if err := cursor.Err(); err != nil {
		t.Fatal(err)
	}

	claim, ok := indexes["pending_token_claim_idx"]
	if !ok {
		t.Fatal("pending_token_claim_idx is absent")
	}
	wantClaimKeys := bson.D{
		{Key: "processed", Value: int32(1)},
		{Key: "nextAttemptAt", Value: int32(1)},
		{Key: "processingUntil", Value: int32(1)},
		{Key: "createdAt", Value: int32(1)},
		{Key: "_id", Value: int32(1)},
	}
	if !sameKeySpec(claim.Key, wantClaimKeys) {
		t.Fatalf("claim index keys = %#v, want %#v", claim.Key, wantClaimKeys)
	}

	ttl, ok := indexes["pending_token_completed_ttl_idx"]
	if !ok {
		t.Fatal("pending_token_completed_ttl_idx is absent")
	}
	if ttl.ExpireAfterSeconds != 30*24*60*60 {
		t.Fatalf("completed TTL = %d, want %d", ttl.ExpireAfterSeconds, 30*24*60*60)
	}
}

func TestQueuePotentialTokenContractCanonicalIngressAndDedupe(t *testing.T) {
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	addressBody := strings.Repeat("A", 128)
	txHashBody := strings.Repeat("B", 64)
	blockHashBody := strings.Repeat("C", 64)
	canonicalAddress := "Q" + strings.ToLower(addressBody)
	canonicalTxHash := "0x" + strings.ToLower(txHashBody)
	cleanup := func() {
		_, _ = collection.DeleteMany(context.Background(), bson.M{"txHash": canonicalTxHash})
	}
	cleanup()
	defer cleanup()

	tx := &models.Transaction{
		Hash:        "0X" + txHashBody,
		BlockNumber: "0X000A",
		BlockHash:   "0X" + blockHashBody,
	}
	if err := QueuePotentialTokenContract("0X"+addressBody, tx, "0X000F"); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.UpdateOne(ctx, bson.M{"txHash": canonicalTxHash}, bson.M{
		"$set":         bson.M{"processed": true},
		"$currentDate": bson.M{"processedAt": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := QueuePotentialTokenContract(canonicalAddress, tx, "0xf"); err != nil {
		t.Fatal(err)
	}

	count, err := collection.CountDocuments(ctx, bson.M{"txHash": canonicalTxHash})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("canonical queue row count = %d, want 1", count)
	}
	var row struct {
		ContractAddress string    `bson:"contractAddress"`
		TxHash          string    `bson:"txHash"`
		BlockNumber     string    `bson:"blockNumber"`
		BlockHash       string    `bson:"blockHash"`
		BlockTimestamp  string    `bson:"blockTimestamp"`
		Processed       bool      `bson:"processed"`
		ProcessedAt     time.Time `bson:"processedAt"`
		CreatedAt       time.Time `bson:"createdAt"`
		UpdatedAt       time.Time `bson:"updatedAt"`
	}
	if err := collection.FindOne(ctx, bson.M{"txHash": canonicalTxHash}).Decode(&row); err != nil {
		t.Fatal(err)
	}
	if row.ContractAddress != canonicalAddress || row.TxHash != canonicalTxHash ||
		row.BlockNumber != "0xa" ||
		row.BlockHash != "0x"+strings.ToLower(blockHashBody) ||
		row.BlockTimestamp != "0xf" {
		t.Fatalf("queue row identity = %+v", row)
	}
	if row.Processed || !row.ProcessedAt.IsZero() {
		t.Fatalf("requeued row retained terminal state: %+v", row)
	}
	if row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		t.Fatalf("queue timestamps are incomplete: %+v", row)
	}
}

func TestPendingTokenCanonicalBlockFenceUsesStoredBSONIdentity(t *testing.T) {
	work, receipt := pendingTokenIdentityFixture()
	collection := configs.GetCollection(configs.DB, "blocks_duplicate_fence_integration")
	previousBlocksCollection := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() {
		configs.BlocksCollections = previousBlocksCollection
		_ = collection.Drop(context.Background())
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanup := func() {
		_, _ = collection.DeleteMany(context.Background(), bson.M{
			"result.hash": exactCaseInsensitiveRegex(work.BlockHash),
		})
	}
	cleanup()
	defer cleanup()

	block := models.ZondDatabaseBlockWithInt{
		BlockNumberInt: 10,
		IngestionState: BlockIngestionComplete,
	}
	block.Result.Number = work.BlockNumber
	block.Result.Hash = work.BlockHash
	block.Result.Timestamp = work.BlockTimestamp
	block.Result.Transactions = []models.Transaction{{
		Hash:        work.TxHash,
		BlockNumber: work.BlockNumber,
		BlockHash:   work.BlockHash,
	}}
	if _, err := collection.InsertOne(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := validatePendingTokenCanonicalBlock(work, receipt); err != nil {
		t.Fatalf("complete canonical block rejected: %v", err)
	}
	if _, err := collection.InsertOne(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := validatePendingTokenCanonicalBlock(work, receipt); err == nil ||
		!strings.Contains(err.Error(), "matches multiple complete canonical blocks") {
		t.Fatalf("duplicate canonical block fence error = %v", err)
	}
	if _, err := collection.DeleteOne(ctx, bson.M{
		"result.hash": exactCaseInsensitiveRegex(work.BlockHash),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.UpdateOne(ctx, bson.M{
		"result.hash": exactCaseInsensitiveRegex(work.BlockHash),
	}, bson.M{"$set": bson.M{"ingestionState": BlockIngestionPending}}); err != nil {
		t.Fatal(err)
	}
	if err := validatePendingTokenCanonicalBlock(work, receipt); err == nil ||
		!strings.Contains(err.Error(), "absent from a complete canonical block") {
		t.Fatalf("pending block fence error = %v", err)
	}
}

func TestPendingTokenClaimFailureCrashAndCompletionLifecycle(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_QUEUE_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_QUEUE_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	collection := client.Database("qrldata-z").Collection("pending_token_claim_integration")
	if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
		t.Fatal(err)
	}
	defer collection.DeleteMany(context.Background(), bson.M{})

	if _, err := collection.InsertOne(ctx, bson.M{
		"contractAddress": "Qcontract",
		"txHash":          "0xtx",
		"blockNumber":     "0x10",
		"blockTimestamp":  "0x20",
		"processed":       false,
	}); err != nil {
		t.Fatal(err)
	}

	claim := func(token string) (pendingTokenContractWork, error) {
		claimCtx, claimCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer claimCancel()
		var work pendingTokenContractWork
		err := collection.FindOneAndUpdate(
			claimCtx,
			pendingTokenClaimFilter(),
			pendingTokenClaimUpdate(token),
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&work)
		return work, err
	}

	first, err := claim("claim-one")
	if err != nil {
		t.Fatal(err)
	}
	if first.ProcessingToken != "claim-one" || first.Attempts != 1 {
		t.Fatalf("first claim = %+v", first)
	}
	if _, err := claim("claim-blocked"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("overlapping claim error = %v, want ErrNoDocuments", err)
	}

	failureResult, err := collection.UpdateOne(
		ctx,
		pendingTokenClaimOwnerFilter(first),
		pendingTokenFailureUpdate("receipt unavailable", 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if failureResult.MatchedCount != 1 {
		t.Fatalf("failure release matched %d rows, want 1", failureResult.MatchedCount)
	}

	second, err := claim("claim-two")
	if err != nil {
		t.Fatal(err)
	}
	if second.ProcessingToken != "claim-two" || second.Attempts != 2 {
		t.Fatalf("second claim = %+v", second)
	}
	staleResult, err := collection.UpdateOne(
		ctx,
		pendingTokenClaimOwnerFilter(first),
		pendingTokenSuccessUpdate(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if staleResult.MatchedCount != 0 {
		t.Fatalf("stale claim completed %d rows, want 0", staleResult.MatchedCount)
	}

	// Simulate a worker crash by leaving processing=true and expiring only its
	// server-visible claim deadline. The next worker must reclaim the row.
	if _, err := collection.UpdateOne(
		ctx,
		pendingTokenClaimOwnerFilter(second),
		bson.M{"$set": bson.M{"processingUntil": time.Unix(0, 0).UTC()}},
	); err != nil {
		t.Fatal(err)
	}
	third, err := claim("claim-three")
	if err != nil {
		t.Fatal(err)
	}
	if third.ProcessingToken != "claim-three" || third.Attempts != 3 {
		t.Fatalf("reclaimed work = %+v", third)
	}

	completeResult, err := collection.UpdateOne(
		ctx,
		pendingTokenClaimOwnerFilter(third),
		pendingTokenSuccessUpdate(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if completeResult.MatchedCount != 1 {
		t.Fatalf("completion matched %d rows, want 1", completeResult.MatchedCount)
	}
	if _, err := claim("claim-after-completion"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("completed row claim error = %v, want ErrNoDocuments", err)
	}

	var completed struct {
		Processed       bool      `bson:"processed"`
		ProcessedAt     time.Time `bson:"processedAt"`
		ProcessingToken string    `bson:"processingToken"`
	}
	if err := collection.FindOne(ctx, bson.M{"txHash": "0xtx"}).Decode(&completed); err != nil {
		t.Fatal(err)
	}
	if !completed.Processed || completed.ProcessedAt.IsZero() || completed.ProcessingToken != "" {
		t.Fatalf("completed row = %+v", completed)
	}
}
