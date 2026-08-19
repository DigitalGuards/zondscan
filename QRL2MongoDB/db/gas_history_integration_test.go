//go:build integration

// Integration coverage for the gas-history timeseries. The behaviour under
// test is an interaction between a Mongo sort/limit and a delete, which a
// unit test cannot fake, so this needs a live MongoDB:
//
//	mongod --dbpath /tmp/mongo-gas-test --port 28017 &
//	MONGOURI=mongodb://127.0.0.1:28017 go test -tags integration -run GasHistory ./db/
//
// The tests use throwaway block numbers inside qrldata-z and clean up after
// themselves, but do NOT point them at a production Mongo.
package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"QRL2MongoDB/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Block numbers used by these tests. High enough not to collide with any
// real block a developer may have synced into a scratch database.
const (
	gasITFirstBlock = int64(900000)
	gasITBlockCount = int64(60)

	// gasITAncientCount must exceed gasHistoryScanLimit. That is the whole
	// point of the regression below: the bug only appears once a single
	// oldest-first scan can be filled entirely with blocks past the
	// retention horizon, so no recent block survives to advance the
	// highwater. Deriving it from the constant keeps the test honest if the
	// limit is ever retuned.
	gasITAncientCount = int64(gasHistoryScanLimit) + 20
)

func gasITCleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	filter := bson.M{"blockNumberInt": bson.M{"$gte": gasITFirstBlock}}
	if _, err := configs.BlocksCollections.DeleteMany(ctx, filter); err != nil {
		t.Fatalf("cleanup blocks: %v", err)
	}
	col := configs.GetCollection(configs.DB, GasHistoryCollection)
	if _, err := col.DeleteMany(ctx, bson.M{"_id": bson.M{"$gte": gasITFirstBlock}}); err != nil {
		t.Fatalf("cleanup gas history: %v", err)
	}
}

// seedBlocks writes count blocks ending at the given time, spaced by
// slotSeconds. Field names and hex encoding match what the syncer writes.
func seedBlocks(t *testing.T, firstBlock, count int64, newestAt time.Time, slotSeconds int64) {
	t.Helper()
	ctx := context.Background()
	docs := make([]interface{}, 0, count)
	for i := int64(0); i < count; i++ {
		blockNumber := firstBlock + i
		age := (count - 1 - i) * slotSeconds
		ts := newestAt.Add(-time.Duration(age) * time.Second).Unix()
		docs = append(docs, bson.M{
			"blockNumberInt": blockNumber,
			"result": bson.M{
				"number":        fmt.Sprintf("0x%x", blockNumber),
				"timestamp":     fmt.Sprintf("0x%x", ts),
				"gasused":       "0x0",
				"gaslimit":      "0x1312d00",
				"basefeepergas": "0x3b9aca00",
				"transactions":  []interface{}{},
			},
		})
	}
	if _, err := configs.BlocksCollections.InsertMany(ctx, docs); err != nil {
		t.Fatalf("seed blocks: %v", err)
	}
}

func gasHistoryHighwater(t *testing.T) int64 {
	t.Helper()
	col := configs.GetCollection(configs.DB, GasHistoryCollection)
	opts := options.FindOne().
		SetProjection(bson.M{"blockNumberInt": 1}).
		SetSort(bson.M{"blockNumberInt": -1})
	var row struct {
		BlockNumberInt int64 `bson:"blockNumberInt"`
	}
	if err := col.FindOne(context.Background(), bson.M{}, opts).Decode(&row); err != nil {
		return -1
	}
	return row.BlockNumberInt
}

func gasHistoryCount(t *testing.T) int64 {
	t.Helper()
	col := configs.GetCollection(configs.DB, GasHistoryCollection)
	n, err := col.CountDocuments(context.Background(), bson.M{"_id": bson.M{"$gte": gasITFirstBlock}})
	if err != nil {
		t.Fatalf("count gas history: %v", err)
	}
	return n
}

// TestGasHistoryKeepsRecentBlocksWhenOlderHistoryExists is the regression
// test for the deadlock that left /gas with an empty chart.
//
// The chain here has blocks far older than GasHistoryRetention sitting below
// recent ones, which is every long-running chain. Scanning oldest-first wrote
// only the ancient blocks, and the retention sweep at the end of the same run
// deleted every one of them, so the highwater reset to -1 and each run redid
// the identical batch forever.
func TestGasHistoryKeepsRecentBlocksWhenOlderHistoryExists(t *testing.T) {
	gasITCleanup(t)
	defer gasITCleanup(t)

	now := time.Now()
	// Enough ancient blocks to fill an entire scan on their own, which is
	// what a chain with months of history actually looks like.
	seedBlocks(t, gasITFirstBlock, gasITAncientCount, now.Add(-90*24*time.Hour), 60)
	// Recent blocks, inside the retention window.
	recentFirst := gasITFirstBlock + gasITAncientCount
	seedBlocks(t, recentFirst, gasITBlockCount, now, 60)

	if err := UpdateGasHistory(); err != nil {
		t.Fatalf("first run: %v", err)
	}

	count := gasHistoryCount(t)
	if count == 0 {
		t.Fatal("first run stored nothing; every written row was swept as too old")
	}
	highwater := gasHistoryHighwater(t)
	if highwater < recentFirst {
		t.Fatalf("highwater = %d, want a recent block (>= %d)", highwater, recentFirst)
	}

	// The second run is where the deadlock showed: it must build on the
	// first rather than start over from an emptied collection.
	if err := UpdateGasHistory(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := gasHistoryHighwater(t); got < highwater {
		t.Fatalf("highwater went backwards: %d then %d", highwater, got)
	}
	if got := gasHistoryCount(t); got < count {
		t.Fatalf("row count shrank across runs: %d then %d", count, got)
	}
}

// TestGasHistorySweepsRowsOlderThanRetention keeps the sweep honest: the fix
// above must not be achieved by simply never deleting anything.
func TestGasHistorySweepsRowsOlderThanRetention(t *testing.T) {
	gasITCleanup(t)
	defer gasITCleanup(t)

	now := time.Now()
	seedBlocks(t, gasITFirstBlock, gasITBlockCount, now, 60)
	if err := UpdateGasHistory(); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	// Plant a row older than the retention horizon and confirm the next run
	// removes it.
	col := configs.GetCollection(configs.DB, GasHistoryCollection)
	stale := gasITFirstBlock + 10_000
	_, err := col.InsertOne(context.Background(), bson.M{
		"_id":            stale,
		"blockNumberInt": stale,
		"timestamp":      now.Add(-GasHistoryRetention - 24*time.Hour).Unix(),
		"gasUsed":        "0x0",
		"gasLimit":       "0x1312d00",
		"baseFeePerGas":  "0x3b9aca00",
		"txCount":        0,
	})
	if err != nil {
		t.Fatalf("insert stale row: %v", err)
	}

	if err := UpdateGasHistory(); err != nil {
		t.Fatalf("sweep run: %v", err)
	}
	n, err := col.CountDocuments(context.Background(), bson.M{"_id": stale})
	if err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if n != 0 {
		t.Fatal("a row older than GasHistoryRetention survived the sweep")
	}
}
