//go:build integration

package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestStoreTokenTransferRepairsLegacyBlockIdentity(t *testing.T) {
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
	collection := client.Database("qrldata-z").Collection("tokenTransfers")
	if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
		t.Fatal(err)
	}
	previousDB := configs.DB
	configs.DB = client
	defer func() {
		configs.DB = previousDB
		_, _ = collection.DeleteMany(context.Background(), bson.M{})
	}()

	txHash := "0x" + strings.Repeat("a", 64)
	contract := "Q" + strings.Repeat("b", 128)
	from := "Q" + strings.Repeat("c", 128)
	to := "Q" + strings.Repeat("d", 128)
	if _, err := collection.InsertMany(ctx, []interface{}{
		bson.M{
			"txHash":          txHash,
			"contractAddress": contract,
			"logIndex":        "0x0",
			"from":            from,
			"to":              to,
			"blockNumber":     "0x1",
		},
		bson.M{
			"txHash":          txHash,
			"contractAddress": contract,
			"from":            from,
			"to":              to,
			"blockNumber":     "0x1",
			"transferType":    "event",
		},
	}); err != nil {
		t.Fatal(err)
	}

	transfer := models.TokenTransfer{
		ContractAddress: contract,
		From:            from,
		To:              to,
		Amount:          "1",
		BlockNumber:     "0x2",
		BlockHash:       "0x" + strings.Repeat("e", 64),
		TxHash:          txHash,
		LogIndex:        "0x0",
		Timestamp:       "0x100",
		TransferType:    "event",
		TokenStandard:   rpc.StandardERC20,
	}
	if err := StoreTokenTransfer(transfer); err != nil {
		t.Fatal(err)
	}
	transfer.BlockNumber = "0x3"
	transfer.BlockHash = "0x" + strings.Repeat("f", 64)
	if err := StoreTokenTransfer(transfer); err != nil {
		t.Fatal(err)
	}

	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("transfer row count = %d, want 1", count)
	}
	var stored models.TokenTransfer
	if err := collection.FindOne(ctx, bson.M{"txHash": txHash}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.BlockNumber != "0x3" || stored.BlockHash != transfer.BlockHash || stored.BlockNumberInt != 3 {
		t.Fatalf("repaired transfer = %+v", stored)
	}
}

func TestStoreTokenEventDeadLetterIsIdempotentAndServerTimestamped(t *testing.T) {
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
	collection := client.Database("qrldata-z").Collection(configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION)
	if err := collection.Drop(ctx); err != nil && !strings.Contains(err.Error(), "ns not found") {
		t.Fatal(err)
	}
	previousDB := configs.DB
	previousDeadLetters := configs.TokenEventDeadLettersCollection
	configs.DB = client
	configs.TokenEventDeadLettersCollection = collection
	defer func() {
		configs.DB = previousDB
		configs.TokenEventDeadLettersCollection = previousDeadLetters
		_ = collection.Drop(context.Background())
	}()
	if err := InitializeTokenEventDeadLettersCollection(); err != nil {
		t.Fatal(err)
	}

	deadLetter := tokenDeadLetterFixture()
	deadLetter.Reason = "first deterministic decode rejection"
	if err := StoreTokenEventDeadLetter(deadLetter); err != nil {
		t.Fatal(err)
	}
	var first models.TokenEventDeadLetter
	if err := collection.FindOne(ctx, bson.M{}).Decode(&first); err != nil {
		t.Fatal(err)
	}
	deadLetter.Reason = "same immutable event replayed"
	if err := StoreTokenEventDeadLetter(deadLetter); err != nil {
		t.Fatal(err)
	}
	if err := InitializeTokenEventDeadLettersCollection(); err != nil {
		t.Fatalf("idempotent index readiness: %v", err)
	}
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dead-letter count = %d, want 1", count)
	}
	var replayed models.TokenEventDeadLetter
	if err := collection.FindOne(ctx, bson.M{}).Decode(&replayed); err != nil {
		t.Fatal(err)
	}
	if replayed.BlockNumberInt != 42 || replayed.Reason != deadLetter.Reason ||
		replayed.FirstSeenAt.IsZero() || replayed.LastSeenAt.IsZero() ||
		!replayed.FirstSeenAt.Equal(first.FirstSeenAt) || replayed.LastSeenAt.Before(replayed.FirstSeenAt) {
		t.Fatalf("replayed dead-letter = %+v, first = %+v", replayed, first)
	}
}

func TestTokenEventDeadLetterIndexReadinessFailsClosedOnConflict(t *testing.T) {
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
	collection := client.Database("qrldata-z").Collection(configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION)
	_ = collection.Drop(ctx)
	defer collection.Drop(context.Background())
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "wrongField", Value: 1}},
		Options: options.Index().SetName("token_event_dead_letter_identity_idx"),
	}); err != nil {
		t.Fatal(err)
	}
	previousDB := configs.DB
	previousDeadLetters := configs.TokenEventDeadLettersCollection
	configs.DB = client
	configs.TokenEventDeadLettersCollection = collection
	defer func() {
		configs.DB = previousDB
		configs.TokenEventDeadLettersCollection = previousDeadLetters
	}()
	if err := InitializeTokenEventDeadLettersCollection(); err == nil ||
		!strings.Contains(err.Error(), "dead-letter indexes") {
		t.Fatalf("conflicting dead-letter index error = %v", err)
	}
}

func TestExactTokenBalanceObservationReplacesOrphanAtSameHeight(t *testing.T) {
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
	collection := client.Database("qrldata-z").Collection("tokenBalances")
	contract := "Q" + strings.Repeat("7", 128)
	holder := "Q" + strings.Repeat("8", 128)
	filter := bson.M{"contractAddress": contract, "holderAddress": holder}
	if _, err := collection.DeleteMany(ctx, filter); err != nil {
		t.Fatal(err)
	}
	defer collection.DeleteMany(context.Background(), filter)
	previousDB := configs.DB
	configs.DB = client
	defer func() { configs.DB = previousDB }()

	oldHash := "0x" + strings.Repeat("a", 64)
	canonicalHash := "0x" + strings.Repeat("b", 64)
	if err := storeTokenBalanceWithGetter(
		contract,
		holder,
		"",
		"0x2a",
		oldHash,
		true,
		func(string, string) (string, error) { return "1", nil },
	); err != nil {
		t.Fatal(err)
	}
	if err := storeTokenBalanceWithGetter(
		contract,
		holder,
		"",
		"0x2a",
		canonicalHash,
		true,
		func(string, string) (string, error) { return "2", nil },
	); err != nil {
		t.Fatal(err)
	}

	olderGetterCalled := false
	if err := storeTokenBalanceWithGetter(
		contract,
		holder,
		"",
		"0x29",
		"0x"+strings.Repeat("c", 64),
		true,
		func(string, string) (string, error) {
			olderGetterCalled = true
			return "3", nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if olderGetterCalled {
		t.Fatal("strictly older observation reached RPC after canonical same-height repair")
	}
	var stored models.TokenBalance
	if err := collection.FindOne(ctx, filter).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.BlockNumber != "0x2a" || stored.BlockNumberInt != 42 ||
		stored.BlockHash != canonicalHash || stored.Balance != "2" {
		t.Fatalf("same-height canonical repair = %+v", stored)
	}
	if _, err := collection.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"balanceStale": true}}); err != nil {
		t.Fatal(err)
	}
	staleGetterCalled := false
	staleReplacementHash := "0x" + strings.Repeat("d", 64)
	if err := storeTokenBalanceWithGetter(
		contract,
		holder,
		"",
		"0x29",
		staleReplacementHash,
		true,
		func(string, string) (string, error) {
			staleGetterCalled = true
			return "3", nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if !staleGetterCalled {
		t.Fatal("rollback-stale higher observation blocked exact canonical replacement")
	}
	if err := collection.FindOne(ctx, filter).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.BlockNumber != "0x29" || stored.BlockNumberInt != 41 ||
		stored.BlockHash != staleReplacementHash || stored.Balance != "3" {
		t.Fatalf("stale higher observation repair = %+v", stored)
	}
	if count, err := collection.CountDocuments(ctx, bson.M{
		"contractAddress": contract,
		"holderAddress":   holder,
		"balanceStale":    true,
	}); err != nil || count != 0 {
		t.Fatalf("stale marker after exact replacement: count=%d err=%v", count, err)
	}
}
