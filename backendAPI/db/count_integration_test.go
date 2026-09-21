package db

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// This opt-in check only reads an existing loopback backup collection. It
// never creates, updates, drops, or indexes data.
func TestCountDocumentsResilientAgainstReadOnlyMongo(t *testing.T) {
	uri := os.Getenv("COUNT_READONLY_MONGO_URI")
	if uri == "" {
		t.Skip("set COUNT_READONLY_MONGO_URI, COUNT_READONLY_MONGO_DB and COUNT_READONLY_EXPECTED_BLOCKS")
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mongodb" || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("count integration check requires an explicit loopback MongoDB address")
	}
	databaseName := os.Getenv("COUNT_READONLY_MONGO_DB")
	if !strings.HasPrefix(databaseName, "receipt_backup_") {
		t.Fatal("count integration check requires a receipt_backup_ database")
	}
	expected, err := strconv.ParseInt(os.Getenv("COUNT_READONLY_EXPECTED_BLOCKS"), 10, 64)
	if err != nil || expected < 0 {
		t.Fatal("COUNT_READONLY_EXPECTED_BLOCKS must be a nonnegative integer")
	}

	var monitorMu sync.Mutex
	var countCommands []string
	var aggregates []bson.Raw
	monitor := &event.CommandMonitor{Started: func(_ context.Context, event *event.CommandStartedEvent) {
		if event.DatabaseName != databaseName {
			return
		}
		monitorMu.Lock()
		defer monitorMu.Unlock()
		countCommands = append(countCommands, event.CommandName)
		if event.CommandName == "aggregate" {
			aggregates = append(aggregates, append(bson.Raw(nil), event.Command...))
		}
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetDirect(true).SetRetryReads(false).SetMonitor(monitor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		if err := client.Disconnect(closeCtx); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	collection := client.Database(databaseName).Collection("blocks")
	started := time.Now()
	count, err := countDocumentsResilient(ctx, collection)
	elapsed := time.Since(started)
	if err != nil || count != expected {
		t.Fatalf("count = %d, %v; want %d, nil", count, err, expected)
	}
	t.Logf("exact indexed count: %d blocks in %s", count, elapsed)

	monitorMu.Lock()
	observedCommands := append([]string(nil), countCommands...)
	observedAggregates := append([]bson.Raw(nil), aggregates...)
	monitorMu.Unlock()
	if len(observedCommands) != 1 || observedCommands[0] != "aggregate" || len(observedAggregates) != 1 {
		t.Fatalf("count commands = %v; want one exact aggregate and no metadata command", observedCommands)
	}
	if hint, ok := observedAggregates[0].Lookup("hint").StringValueOK(); !ok || hint != "_id_" {
		t.Fatalf("count hint = %q; want _id_", hint)
	}
	if bytes.Contains(observedAggregates[0], []byte("$collStats")) {
		t.Fatal("exact count used collection metadata")
	}
	expired, expireCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expireCancel()
	if _, err := countDocumentsResilient(expired, collection); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired caller deadline returned %v", err)
	}
}
