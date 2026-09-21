//go:build integration

package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
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

func integrationTokenBlock(number string, numberInt int64, withTransaction bool) models.ZondDatabaseBlockWithInt {
	blockHash := "0x" + strings.Repeat(string(rune('a'+numberInt)), 64)
	block := models.ZondDatabaseBlockWithInt{
		BlockNumberInt:      numberInt,
		IngestionState:      db.BlockIngestionComplete,
		TokenIngestionState: TokenIngestionPending,
	}
	block.Result.Number = number
	block.Result.Hash = blockHash
	block.Result.Timestamp = "0x100"
	block.Result.TransactionsRoot = "0x" + strings.Repeat("f", 64)
	if withTransaction {
		block.Result.Transactions = []models.Transaction{{
			Hash:             "0x" + strings.Repeat(string(rune('1'+numberInt)), 64),
			BlockNumber:      number,
			BlockHash:        blockHash,
			TransactionIndex: "0x0",
		}}
	}
	return block
}

func TestTokenBlockClaimCrashRetryAndCompletionLifecycle(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	collection := client.Database("zondscan_token_block_integration").Collection("blocks")
	if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
		t.Fatal(err)
	}
	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() {
		configs.BlocksCollections = previous
		_, _ = collection.DeleteMany(context.Background(), bson.M{})
	}()

	block := integrationTokenBlock("0x1", 1, true)
	if _, err := collection.InsertOne(ctx, block); err != nil {
		t.Fatal(err)
	}
	first, err := claimTokenBlock(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.TokenAttempts != 1 || first.TokenProcessingToken == "" {
		t.Fatalf("first claim = %+v", first)
	}
	if _, err := claimTokenBlock(ctx, ""); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("live claim was concurrently reclaimable: %v", err)
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": first.ID}, bson.M{
		"$set": bson.M{"tokenProcessingUntil": time.Unix(0, 0).UTC()},
	}); err != nil {
		t.Fatal(err)
	}
	second, err := claimTokenBlock(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.TokenAttempts != 2 || second.TokenProcessingToken == first.TokenProcessingToken {
		t.Fatalf("crash reclaim = %+v", second)
	}
	staleErr := errors.New("stale owner must not release successor claim")
	if err := releaseFailedTokenBlockClaim(first, staleErr); err == nil ||
		!strings.Contains(err.Error(), "ownership was lost") {
		t.Fatalf("stale owner release error = %v", err)
	}
	var currentOwner struct {
		Token string `bson:"tokenProcessingToken"`
	}
	if err := collection.FindOne(ctx, bson.M{"_id": second.ID}).Decode(&currentOwner); err != nil {
		t.Fatal(err)
	}
	if currentOwner.Token != second.TokenProcessingToken {
		t.Fatalf("stale release changed successor owner: %+v", currentOwner)
	}

	retryErr := errors.New("retryable qrl_getLogs failure")
	if err := releaseFailedTokenBlockClaim(second, retryErr); !errors.Is(err, retryErr) {
		t.Fatalf("failure release error = %v", err)
	}
	var failed struct {
		State         string    `bson:"tokenIngestionState"`
		LastError     string    `bson:"lastTokenError"`
		LastFailedAt  time.Time `bson:"lastTokenFailedAt"`
		NextAttemptAt time.Time `bson:"tokenNextAttemptAt"`
		Token         string    `bson:"tokenProcessingToken"`
	}
	if err := collection.FindOne(ctx, bson.M{"_id": second.ID}).Decode(&failed); err != nil {
		t.Fatal(err)
	}
	if failed.State != TokenIngestionPending || failed.LastError != retryErr.Error() ||
		failed.LastFailedAt.IsZero() || failed.NextAttemptAt.IsZero() || failed.Token != "" {
		t.Fatalf("failed claim row = %+v", failed)
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": second.ID}, bson.M{
		"$set": bson.M{"tokenNextAttemptAt": time.Unix(0, 0).UTC()},
	}); err != nil {
		t.Fatal(err)
	}
	third, err := claimTokenBlock(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := tokenBlockSnapshot(third.block())
	if err != nil {
		t.Fatal(err)
	}
	if err := completeTokenBlockClaim(third, snapshot); err != nil {
		t.Fatal(err)
	}
	var completed struct {
		State       string    `bson:"tokenIngestionState"`
		CompletedAt time.Time `bson:"tokenCompletedAt"`
		Token       string    `bson:"tokenProcessingToken"`
	}
	if err := collection.FindOne(ctx, bson.M{"_id": third.ID}).Decode(&completed); err != nil {
		t.Fatal(err)
	}
	if completed.State != TokenIngestionComplete || completed.CompletedAt.IsZero() || completed.Token != "" {
		t.Fatalf("completed claim row = %+v", completed)
	}
	duplicate := block
	duplicate.Result.Hash = "0x" + strings.Repeat("9", 64)
	duplicate.Result.Transactions[0].BlockHash = duplicate.Result.Hash
	if _, err := collection.InsertOne(ctx, duplicate); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUniqueCompleteTokenBlock(ctx, block.Result.Number, block.Result.Hash); err == nil ||
		!strings.Contains(err.Error(), "not unique") {
		t.Fatalf("duplicate-height block error = %v", err)
	}
}

func TestTokenBlockStateInitializationHandlesLegacyAndEmptyRows(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database("zondscan_token_block_integration").Collection("blocks")
	if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
		t.Fatal(err)
	}
	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() {
		configs.BlocksCollections = previous
		_, _ = collection.DeleteMany(context.Background(), bson.M{})
	}()

	legacy := integrationTokenBlock("0x2", 2, true)
	legacy.TokenIngestionState = ""
	empty := integrationTokenBlock("0x3", 3, false)
	if _, err := collection.InsertMany(ctx, []interface{}{legacy, empty}); err != nil {
		t.Fatal(err)
	}
	if err := initializeTokenBlockStates(ctx); err != nil {
		t.Fatal(err)
	}
	var states []struct {
		Number string `bson:"result.number"`
		State  string `bson:"tokenIngestionState"`
	}
	cursor, err := collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "blockNumberInt", Value: 1}}))
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].State != TokenIngestionPending || states[1].State != TokenIngestionComplete {
		t.Fatalf("initialized states = %+v", states)
	}
}

func TestOldestPendingTokenBlockBackoffBlocksHigherClaims(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database("zondscan_token_block_integration").Collection("blocks_oldest_order")
	_ = collection.Drop(ctx)
	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() {
		configs.BlocksCollections = previous
		_ = collection.Drop(context.Background())
	}()
	if err := initializeTokenBlockQueueIndexes(); err != nil {
		t.Fatal(err)
	}

	lower := integrationTokenBlock("0x4", 4, true)
	higher := integrationTokenBlock("0x5", 5, true)
	futureAttempt := time.Now().UTC().Add(time.Hour)
	lower.TokenNextAttemptAt = &futureAttempt
	if _, err := collection.InsertMany(ctx, []interface{}{higher, lower}); err != nil {
		t.Fatal(err)
	}
	if err := ProcessTokenTransfersForBlock("0x5"); err != nil {
		t.Fatalf("defer higher direct token processing: %v", err)
	}
	if _, err := claimOldestPendingTokenBlock(ctx); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("higher block bypassed lower backoff: %v", err)
	}
	var higherRow tokenBlockWork
	if err := collection.FindOne(ctx, bson.M{"blockNumberInt": int64(5)}).Decode(&higherRow); err != nil {
		t.Fatal(err)
	}
	if higherRow.TokenProcessingToken != "" || higherRow.TokenAttempts != 0 {
		t.Fatalf("higher block was claimed while lower was backed off: %+v", higherRow)
	}
	incomplete, err := tokenBlockQueueHasIncompleteWork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !incomplete {
		t.Fatal("balance reconciliation gate missed pending token blocks")
	}
	if _, err := collection.UpdateOne(ctx, bson.M{"blockNumberInt": int64(4)}, bson.M{
		"$set": bson.M{"tokenNextAttemptAt": time.Unix(0, 0).UTC()},
	}); err != nil {
		t.Fatal(err)
	}
	claimedLower, err := claimOldestPendingTokenBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if claimedLower.BlockNumberInt != 4 {
		t.Fatalf("claimed block = %d, want 4", claimedLower.BlockNumberInt)
	}
	snapshot, err := tokenBlockSnapshot(claimedLower.block())
	if err != nil {
		t.Fatal(err)
	}
	if err := completeTokenBlockClaim(claimedLower, snapshot); err != nil {
		t.Fatal(err)
	}
	claimedHigher, err := claimOldestPendingTokenBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if claimedHigher.BlockNumberInt != 5 {
		t.Fatalf("claimed block = %d, want 5", claimedHigher.BlockNumberInt)
	}
}

func TestTokenBlockQueueIndexInitializationFailsClosedOnConflict(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database("zondscan_token_block_integration").Collection("blocks_index_conflict")
	_ = collection.Drop(ctx)
	defer collection.Drop(context.Background())
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "wrongField", Value: 1}},
		Options: options.Index().SetName("token_ingestion_claim_idx"),
	}); err != nil {
		t.Fatal(err)
	}
	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() { configs.BlocksCollections = previous }()
	if err := initializeTokenBlockQueueIndexes(); err == nil {
		t.Fatal("conflicting token claim index was accepted")
	}
}

func TestTokenBlockStateInitializationRejectsUnknownState(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database("zondscan_token_block_integration").Collection("blocks_invalid_state")
	_ = collection.Drop(ctx)
	defer collection.Drop(context.Background())
	block := integrationTokenBlock("0x4", 4, false)
	block.TokenIngestionState = "wedged"
	if _, err := collection.InsertOne(ctx, block); err != nil {
		t.Fatal(err)
	}
	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	defer func() { configs.BlocksCollections = previous }()
	if err := initializeTokenBlockStates(ctx); err == nil ||
		!strings.Contains(err.Error(), "unsupported tokenIngestionState") ||
		!strings.Contains(err.Error(), "wedged") {
		t.Fatalf("unknown token state error = %v", err)
	}
	incomplete, err := tokenBlockQueueHasIncompleteWork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !incomplete {
		t.Fatal("balance reconciliation gate ignored unknown token state")
	}
}

func TestDeadLetteredPoisonLogAllowsHigherTokenHeightToProgress(t *testing.T) {
	uri := os.Getenv("QRL_TOKEN_BLOCK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_TOKEN_BLOCK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	blockCollection := client.Database("zondscan_token_block_integration").Collection("blocks_poison_progress")
	deadLetterCollection := client.Database("zondscan_token_block_integration").Collection("token_dead_letters_poison_progress")
	_ = blockCollection.Drop(ctx)
	_ = deadLetterCollection.Drop(ctx)
	previousDB := configs.DB
	previousBlocks := configs.BlocksCollections
	previousDeadLetters := configs.TokenEventDeadLettersCollection
	configs.DB = client
	configs.BlocksCollections = blockCollection
	configs.TokenEventDeadLettersCollection = deadLetterCollection
	defer func() {
		configs.DB = previousDB
		configs.BlocksCollections = previousBlocks
		configs.TokenEventDeadLettersCollection = previousDeadLetters
		_ = blockCollection.Drop(context.Background())
		_ = deadLetterCollection.Drop(context.Background())
	}()
	if err := initializeTokenBlockQueueIndexes(); err != nil {
		t.Fatal(err)
	}
	if err := db.InitializeTokenEventDeadLettersCollection(); err != nil {
		t.Fatal(err)
	}

	lower := integrationTokenBlock("0x1", 1, true)
	higher := integrationTokenBlock("0x2", 2, true)
	if _, err := blockCollection.InsertMany(ctx, []interface{}{higher, lower}); err != nil {
		t.Fatal(err)
	}
	claimedLower, err := claimOldestPendingTokenBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	deadLetter := models.TokenEventDeadLetter{
		BlockNumber:   claimedLower.Result.Number,
		BlockHash:     claimedLower.Result.Hash,
		TxHash:        claimedLower.Result.Transactions[0].Hash,
		LogIndex:      "0x0",
		Emitter:       "Q" + strings.Repeat("d", 128),
		Topic0:        rpc.TransferEventSignature,
		TokenStandard: rpc.StandardERC20,
		Reason:        "deterministic token event decode rejected: malformed ABI payload",
	}
	if err := db.StoreTokenEventDeadLetter(deadLetter); err != nil {
		t.Fatal(err)
	}
	snapshot, err := tokenBlockSnapshot(claimedLower.block())
	if err != nil {
		t.Fatal(err)
	}
	if err := completeTokenBlockClaim(claimedLower, snapshot); err != nil {
		t.Fatal(err)
	}
	claimedHigher, err := claimOldestPendingTokenBlock(ctx)
	if err != nil {
		t.Fatalf("higher height remained blocked after durable quarantine: %v", err)
	}
	if claimedHigher.BlockNumberInt != higher.BlockNumberInt {
		t.Fatalf("claimed block = %d, want %d", claimedHigher.BlockNumberInt, higher.BlockNumberInt)
	}
	if err := db.StoreTokenEventDeadLetter(deadLetter); err != nil {
		t.Fatalf("idempotent poison replay: %v", err)
	}
	count, err := deadLetterCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dead-letter count after replay = %d, want 1", count)
	}
}
