package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"backendAPI/configs"
	"backendAPI/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestTransactionListsPreserveCanonicalPaginationAgainstMongo(t *testing.T) {
	uri := os.Getenv("CANONICAL_VISIBILITY_MONGO_URI")
	if uri == "" {
		t.Skip("set CANONICAL_VISIBILITY_MONGO_URI to run the Mongo transaction-list test")
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mongodb" || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("transaction-list fixtures require an explicit loopback MongoDB address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	database := client.Database("zondscan_transaction_order_" + primitive.NewObjectID().Hex())
	previousBlocks, previousTransactions := configs.BlocksCollection, configs.TransactionByAddressCollection
	configs.BlocksCollection = database.Collection("blocks")
	configs.TransactionByAddressCollection = database.Collection("transactionByAddress")
	t.Cleanup(func() {
		configs.BlocksCollection, configs.TransactionByAddressCollection = previousBlocks, previousTransactions
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := database.Drop(cleanupCtx); err != nil {
			t.Errorf("drop isolated fixture database: %v", err)
		}
		_ = client.Disconnect(cleanupCtx)
	})
	insert := func(collection *mongo.Collection, rows []interface{}) {
		t.Helper()
		if _, err := collection.InsertMany(ctx, rows); err != nil {
			t.Fatal(err)
		}
	}
	insert(configs.BlocksCollection, []interface{}{
		bson.M{"result": bson.M{"number": "0x1"}, "ingestionState": "complete"},
		bson.M{"result": bson.M{"number": "0x2"}, "ingestionState": "pending"},
		bson.M{"result": bson.M{"number": "0x4"}, "ingestionState": "complete"},
		bson.M{"result": bson.M{"number": "0x4"}, "ingestionState": "pending"},
		bson.M{"result": bson.M{"number": "0x5"}, "ingestionState": "complete"},
		bson.M{"result": bson.M{"number": "0x5"}, "ingestionState": "complete"},
		bson.M{"result": bson.M{"number": "0x6"}, "ingestionState": "unknown"},
		bson.M{"result": bson.M{"number": "0x7"}},
	})
	for _, indexed := range []struct {
		collection *mongo.Collection
		keys       bson.D
	}{
		{configs.BlocksCollection, bson.D{{Key: "result.number", Value: -1}, {Key: "result.timestamp", Value: 1}}},
		{configs.TransactionByAddressCollection, bson.D{{Key: "timeStamp", Value: -1}}},
	} {
		if _, err := indexed.collection.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: indexed.keys}); err != nil {
			t.Fatal(err)
		}
	}
	var fixtures []interface{}
	hashes := make([]string, 105)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("0x%064x", i+1)
		fixtures = append(fixtures, bson.M{
			"blockNumber": "0x1", "txHash": hashes[i], "timeStamp": fmt.Sprintf("0x%08x", i+100),
			"amountWei": "123", "paidFeesWei": "7", "receiptStatus": "0x1", "address": "counterparty",
		})
	}
	// Newer and interleaved hidden rows must consume neither page offsets nor limits.
	for index, block := range []interface{}{"0x2", "0x3", "0x4", "0x5", "0x6", "0x7", nil} {
		for _, timestamp := range []int{1000 + index, 195 + index} {
			fixtures = append(fixtures, bson.M{"blockNumber": block, "txHash": fmt.Sprintf("hidden-%d-%d", index, timestamp), "timeStamp": fmt.Sprintf("0x%08x", timestamp)})
		}
	}
	fixtures = append(fixtures, bson.M{"txHash": "missing-block-number", "timeStamp": "0xffffffff"})
	insert(configs.TransactionByAddressCollection, fixtures)
	expectHashes := func(rows []models.TransactionByAddress, first, length int) {
		t.Helper()
		if len(rows) != length {
			t.Fatalf("got %d rows, want %d", len(rows), length)
		}
		for i, row := range rows {
			if row.TxHash != hashes[first-i] || row.BlockNumber != "0x1" || row.AmountWei != "123" || row.PaidFeesWei != "7" || row.Status != "0x1" {
				t.Fatalf("row %d = %#v, want unchanged complete transaction %s", i, row, hashes[first-i])
			}
		}
	}
	for _, query := range []struct{ page, limit, first, length int }{
		{1, 10, 104, 10}, {2, 10, 94, 10}, {0, 0, 104, 10}, {1, 500, 104, 100}, {11, 10, 4, 5}, {12, 10, 0, 0},
	} {
		rows, err := ReturnTransactionsNetwork(query.page, query.limit)
		if err != nil {
			t.Fatal(err)
		}
		expectHashes(rows, query.first, query.length)
	}
	latest, err := ReturnLatestTransactions()
	if err != nil {
		t.Fatal(err)
	}
	expectHashes(latest, 104, 100)
	legacyProjection := bson.M{}
	for _, field := range []string{"inOut", "txType", "address", "from", "to", "txHash", "timeStamp", "amount", "amountWei", "paidFees", "paidFeesWei", "feeSource", "receiptStatus", "blockNumber"} {
		legacyProjection[field] = 1
	}
	legacyPipeline := append(canonicalCompanionPipeline(bson.D{}),
		bson.M{"$sort": bson.D{{Key: "timeStamp", Value: -1}}},
		bson.M{"$limit": 100}, bson.M{"$project": legacyProjection},
	)
	legacyCursor, err := configs.TransactionByAddressCollection.Aggregate(ctx, legacyPipeline)
	if err != nil {
		t.Fatal(err)
	}
	defer legacyCursor.Close(ctx)
	var legacyRows []models.TransactionByAddress
	if err := legacyCursor.All(ctx, &legacyRows); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(latest, legacyRows) {
		t.Fatal("optimized latest list changed the previous query's decoded response")
	}
	if count, err := CountTransactionsNetwork(); err != nil || count != 105 {
		t.Fatalf("exact canonical count = %d, %v; want 105", count, err)
	}
	// Timestamps remain the sole public ordering key. Equal timestamps permit
	// either hash order, but both visible rows must survive and hidden rows must not.
	tieHashes := []string{fmt.Sprintf("0x%064x", 1001), fmt.Sprintf("0x%064x", 1002)}
	insert(configs.TransactionByAddressCollection, []interface{}{
		bson.M{"blockNumber": "0x1", "txHash": tieHashes[0], "timeStamp": "0xfffffffe"},
		bson.M{"blockNumber": "0x1", "txHash": tieHashes[1], "timeStamp": "0xfffffffe"},
		bson.M{"blockNumber": "0x2", "txHash": "hidden-tie", "timeStamp": "0xfffffffe"},
	})
	rows, err := ReturnTransactionsNetwork(1, 2)
	if err != nil || len(rows) != 2 {
		t.Fatalf("tie page: %d rows, %v", len(rows), err)
	}
	got := []string{rows[0].TxHash, rows[1].TxHash}
	sort.Strings(got)
	if !reflect.DeepEqual(got, tieHashes) {
		t.Fatalf("tie membership = %v, want %v", got, tieHashes)
	}
	if count, err := CountTransactionsNetwork(); err != nil || count != 107 {
		t.Fatalf("exact canonical count after ties = %d, %v; want 107", count, err)
	}
}
