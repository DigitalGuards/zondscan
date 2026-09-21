//go:build integration

package db

import (
	"QRL2MongoDB/configs"
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestEnsureUniqueBlockHeightIndexUpgradesLegacyIndex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database := configs.DB.Database("zondscan_block_unique_index_test")
	if err := database.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Drop(context.Background())
	})
	collection := database.Collection("blocks")
	if _, err := collection.InsertMany(ctx, []interface{}{
		bson.M{"blockNumberInt": int64(0), "result": bson.M{"number": "0x0"}},
		bson.M{"blockNumberInt": int64(1), "result": bson.M{"number": "0x1"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "blockNumberInt", Value: -1}},
		Options: options.Index().SetName(BlockHeightUniqueIndex),
	}); err != nil {
		t.Fatal(err)
	}

	previous := configs.BlocksCollections
	configs.BlocksCollections = collection
	t.Cleanup(func() {
		configs.BlocksCollections = previous
	})
	if err := EnsureUniqueBlockHeightIndex(); err != nil {
		t.Fatal(err)
	}
	if err := validateUniqueBlockHeightIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := collection.InsertOne(ctx, bson.M{
		"blockNumberInt": int64(1),
		"result":         bson.M{"number": "0x1"},
	}); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate height insert error = %v, want duplicate key", err)
	}
}
