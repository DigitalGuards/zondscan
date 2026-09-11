package db

import (
	"backendAPI/configs"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestBlockIngestionReadinessAgainstMongo(t *testing.T) {
	uri := os.Getenv("CANONICAL_VISIBILITY_MONGO_URI")
	if uri == "" {
		t.Skip("set CANONICAL_VISIBILITY_MONGO_URI to run the Mongo readiness test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	database := client.Database("zondscan_block_readiness_test")
	if err := database.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Drop(context.Background())
	})
	collection := database.Collection("blocks")
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "blockNumberInt", Value: -1}},
		Options: options.Index().SetName(blockHeightUniqueIndexName).SetUnique(true),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.InsertOne(ctx, bson.M{
		"blockNumberInt": int64(1),
		"result":         bson.M{"number": "0x1"},
		"ingestionState": completedBlockIngestionState,
	}); err != nil {
		t.Fatal(err)
	}

	previous := configs.BlocksCollection
	configs.BlocksCollection = collection
	t.Cleanup(func() {
		configs.BlocksCollection = previous
	})
	if err := ValidateBlockIngestionReadiness(ctx); err != nil {
		t.Fatalf("unique complete block rejected: %v", err)
	}
	if _, err := collection.InsertOne(ctx, bson.M{
		"blockNumberInt": int64(1),
		"result":         bson.M{"number": "0x1"},
		"ingestionState": completedBlockIngestionState,
	}); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate height insert error = %v, want duplicate key", err)
	}
	if _, err := collection.Indexes().DropOne(ctx, blockHeightUniqueIndexName); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "blockNumberInt", Value: -1}},
		Options: options.Index().SetName(blockHeightUniqueIndexName),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBlockIngestionReadiness(ctx); err == nil ||
		!strings.Contains(err.Error(), "unique block-height index") {
		t.Fatalf("non-unique readiness error = %v", err)
	}
}
