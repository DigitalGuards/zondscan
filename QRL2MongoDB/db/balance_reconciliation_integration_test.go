package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const balanceReconciliationIntegrationURI = "QRL_BALANCE_RECONCILIATION_INTEGRATION_URI"

func TestBalanceReconciliationFailureRetryAndClaimRecovery(t *testing.T) {
	uri := os.Getenv(balanceReconciliationIntegrationURI)
	if uri == "" {
		t.Skipf("set %s to run the isolated MongoDB queue test", balanceReconciliationIntegrationURI)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
	})
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		t.Fatal(err)
	}

	databaseName := fmt.Sprintf("zondscan_balance_queue_it_%d", os.Getpid())
	database := client.Database(databaseName)
	collection := database.Collection("balance_reconciliations")
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		if err := database.Drop(dropCtx); err != nil {
			t.Errorf("drop isolated balance queue database: %v", err)
		}
	})
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "availableAt", Value: 1},
			{Key: "createdAt", Value: 1},
			{Key: "_id", Value: 1},
		},
	}); err != nil {
		t.Fatal(err)
	}

	addressA := "Q" + strings.Repeat("1", 128)
	addressB := "Q" + strings.Repeat("2", 128)
	addressC := "Q" + strings.Repeat("3", 128)
	addressD := "Q" + strings.Repeat("4", 128)
	addressE := "Q" + strings.Repeat("5", 128)
	failed := balanceIntegrationNativeItem(t, addressA)
	failed.CreatedAt = time.Now().Add(-time.Hour).UTC()
	failed.AvailableAt = time.Now().Add(-time.Minute).UTC()
	if _, err := collection.InsertOne(ctx, failed); err != nil {
		t.Fatal(err)
	}

	failureSentinel := errors.New("canonical RPC unavailable")
	failureStarted := time.Now()
	if err := reconcileStaleBalanceQueue(
		ctx,
		collection,
		1,
		"0x10",
		func(item balanceReconciliation, _ string) error {
			if item.ID != failed.ID {
				t.Fatalf("claimed %q, want oldest failed item %q", item.ID, failed.ID)
			}
			return failureSentinel
		},
	); err != nil {
		t.Fatal(err)
	}

	var failedAfterAttempt balanceReconciliation
	if err := collection.FindOne(ctx, bson.M{"_id": failed.ID}).Decode(&failedAfterAttempt); err != nil {
		t.Fatal(err)
	}
	if failedAfterAttempt.Attempts != 1 || failedAfterAttempt.LastError != failureSentinel.Error() {
		t.Fatalf("failed item state = %+v", failedAfterAttempt)
	}
	if failedAfterAttempt.ClaimToken != "" || failedAfterAttempt.ClaimExpiresAt != (time.Time{}) {
		t.Fatalf("failed item retained claim state: %+v", failedAfterAttempt)
	}
	if failedAfterAttempt.AvailableAt.Before(failureStarted.Add(4*time.Second)) ||
		failedAfterAttempt.AvailableAt.After(time.Now().Add(7*time.Second)) {
		t.Fatalf("first retry deadline %s is outside server-time backoff window",
			failedAfterAttempt.AvailableAt)
	}

	newer := balanceIntegrationNativeItem(t, addressB)
	newer.CreatedAt = time.Now().UTC()
	newer.AvailableAt = time.Now().Add(-time.Second).UTC()
	if _, err := collection.InsertOne(ctx, newer); err != nil {
		t.Fatal(err)
	}
	processed := make([]string, 0, 1)
	if err := reconcileStaleBalanceQueue(
		ctx,
		collection,
		10,
		"0x10",
		func(item balanceReconciliation, _ string) error {
			processed = append(processed, item.ID)
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if len(processed) != 1 || processed[0] != newer.ID {
		t.Fatalf("eligible work processed while failure backed off = %v, want [%s]", processed, newer.ID)
	}
	if count, err := collection.CountDocuments(ctx, bson.M{"_id": newer.ID}); err != nil || count != 0 {
		t.Fatalf("newer successful item count = %d, err=%v", count, err)
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": failed.ID}, bson.M{"$set": bson.M{
		"availableAt":   time.Now().Add(-time.Second).UTC(),
		"nextAttemptAt": time.Now().Add(-time.Second).UTC(),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := reconcileStaleBalanceQueue(
		ctx,
		collection,
		1,
		"0x10",
		func(item balanceReconciliation, _ string) error { return nil },
	); err != nil {
		t.Fatal(err)
	}
	if count, err := collection.CountDocuments(ctx, bson.M{"_id": failed.ID}); err != nil || count != 0 {
		t.Fatalf("successful retry item count = %d, err=%v", count, err)
	}

	reclaimable := balanceIntegrationNativeItem(t, addressC)
	reclaimable.CreatedAt = time.Now().Add(-time.Minute).UTC()
	reclaimable.AvailableAt = time.Now().Add(-time.Second).UTC()
	reclaimable.ClaimToken = "crashed-worker"
	reclaimable.ClaimedAt = time.Now().Add(-2 * balanceReconciliationClaimTTL).UTC()
	reclaimable.ClaimExpiresAt = time.Now().Add(-balanceReconciliationClaimTTL).UTC()
	if _, err := collection.InsertOne(ctx, reclaimable); err != nil {
		t.Fatal(err)
	}
	var reclaimed balanceReconciliation
	if err := reconcileStaleBalanceQueue(
		ctx,
		collection,
		1,
		"0x10",
		func(item balanceReconciliation, _ string) error {
			reclaimed = item
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if reclaimed.ClaimToken == "" || reclaimed.ClaimToken == "crashed-worker" {
		t.Fatalf("expired claim was not replaced: %+v", reclaimed)
	}
	if reclaimed.ClaimedAt.IsZero() ||
		reclaimed.ClaimExpiresAt.Sub(reclaimed.ClaimedAt) != balanceReconciliationClaimTTL ||
		!reclaimed.AvailableAt.Equal(reclaimed.ClaimExpiresAt) {
		t.Fatalf("reclaimed server-time lease is inconsistent: %+v", reclaimed)
	}

	active := balanceIntegrationNativeItem(t, addressD)
	active.CreatedAt = time.Now().Add(-time.Minute).UTC()
	active.AvailableAt = time.Now().Add(time.Minute).UTC()
	active.ClaimToken = "active-worker"
	active.ClaimedAt = time.Now().UTC()
	active.ClaimExpiresAt = active.AvailableAt
	if _, err := collection.InsertOne(ctx, active); err != nil {
		t.Fatal(err)
	}
	activeProcessed := false
	if err := reconcileStaleBalanceQueue(
		ctx,
		collection,
		10,
		"0x10",
		func(item balanceReconciliation, _ string) error {
			activeProcessed = true
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if activeProcessed {
		t.Fatal("active claim was processed before its server-time deadline")
	}

	ownershipChanged := balanceIntegrationNativeItem(t, addressE)
	ownershipChanged.CreatedAt = time.Now().UTC()
	ownershipChanged.AvailableAt = time.Now().Add(-time.Second).UTC()
	if _, err := collection.InsertOne(ctx, ownershipChanged); err != nil {
		t.Fatal(err)
	}
	err = reconcileStaleBalanceQueue(
		ctx,
		collection,
		1,
		"0x10",
		func(item balanceReconciliation, _ string) error {
			_, updateErr := collection.UpdateOne(ctx, bson.M{
				"_id":        item.ID,
				"claimToken": item.ClaimToken,
			}, bson.M{"$set": bson.M{"claimToken": "replacement-owner"}})
			return updateErr
		},
	)
	if !errors.Is(err, errBalanceReconciliationClaimLost) {
		t.Fatalf("ownership-changing success error = %v, want claim-lost sentinel", err)
	}
	if count, err := collection.CountDocuments(ctx, bson.M{
		"_id":        ownershipChanged.ID,
		"claimToken": "replacement-owner",
	}); err != nil || count != 1 {
		t.Fatalf("replacement-owned item count = %d, err=%v", count, err)
	}
}

func balanceIntegrationNativeItem(t *testing.T, address string) balanceReconciliation {
	t.Helper()
	item, ok := newNativeBalanceReconciliation(address, "0x10")
	if !ok {
		t.Fatalf("invalid integration address %q", address)
	}
	return item
}
