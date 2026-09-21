//go:build integration

package configs

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestSyncerLeaseServerTimeLifecycle(t *testing.T) {
	uri := os.Getenv("QRL_SYNCER_LEASE_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_SYNCER_LEASE_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	var hello struct {
		SetName string `bson:"setName"`
		Msg     string `bson:"msg"`
	}
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil {
		t.Fatal(err)
	}
	if !mongoTopologySupportsTransactions(hello.SetName, hello.Msg) {
		t.Fatal("integration MongoDB is not a replica set or mongos")
	}

	collection := client.Database("zondscan_lease_integration").Collection("syncer_leases")
	if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
		t.Fatal(err)
	}
	previous := SyncerLeasesCollection
	SyncerLeasesCollection = collection
	defer func() {
		SyncerLeasesCollection = previous
		_, _ = collection.DeleteMany(context.Background(), bson.M{})
	}()

	first, err := AcquireSyncerLease(ctx, "integration-owner-a", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if first.Generation != 1 || first.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatalf("unexpected first lease: %+v", first)
	}
	if _, err := AcquireSyncerLease(ctx, "integration-owner-b", 5*time.Second); !errors.Is(err, ErrSyncerLeaseHeld) {
		t.Fatalf("second owner acquire error = %v, want ErrSyncerLeaseHeld", err)
	}

	renewed, err := RenewSyncerLease(ctx, first, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.Generation != first.Generation || !renewed.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("unexpected renewed lease: before=%+v after=%+v", first, renewed)
	}
	if err := ReleaseSyncerLease(ctx, renewed); err != nil {
		t.Fatal(err)
	}

	second, err := AcquireSyncerLease(ctx, "integration-owner-b", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if second.Generation != first.Generation+1 {
		t.Fatalf("generation after release = %d, want %d", second.Generation, first.Generation+1)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := AcquireSyncerLease(ctx, "integration-owner-c", 5*time.Second); !errors.Is(err, ErrSyncerLeaseHeld) {
		t.Fatalf("expired unreleased lease acquire error = %v, want ErrSyncerLeaseHeld", err)
	}
	if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, second); err != nil {
		t.Fatal(err)
	}
	third, err := AcquireSyncerLease(ctx, "integration-owner-c", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if third.Generation != second.Generation+1 {
		t.Fatalf("generation after compute-fenced release = %d, want %d",
			third.Generation, second.Generation+1)
	}
	if _, err := RenewSyncerLease(ctx, first, 5*time.Second); !errors.Is(err, ErrSyncerLeaseLost) {
		t.Fatalf("stale owner renewal error = %v, want ErrSyncerLeaseLost", err)
	}
	if err := ReleaseSyncerLease(ctx, third); err != nil {
		t.Fatal(err)
	}
}
