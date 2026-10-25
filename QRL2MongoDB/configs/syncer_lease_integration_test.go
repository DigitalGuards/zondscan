//go:build integration

package configs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func setupSyncerLeaseIntegration(t *testing.T, monitor *event.CommandMonitor) (context.Context, *mongo.Collection) {
	t.Helper()
	uri := os.Getenv("QRL_SYNCER_LEASE_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_SYNCER_LEASE_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMonitor(monitor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect integration MongoDB: %v", err)
		}
	})

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

	collection := client.Database(fmt.Sprintf("zondscan_lease_integration_%d", time.Now().UnixNano())).Collection("syncer_leases")
	previous := SyncerLeasesCollection
	SyncerLeasesCollection = collection
	t.Cleanup(func() {
		SyncerLeasesCollection = previous
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := collection.Database().Drop(cleanupCtx); err != nil {
			t.Errorf("drop integration database: %v", err)
		}
	})
	return ctx, collection
}

func TestSyncerLeaseServerTimeLifecycle(t *testing.T) {
	ctx, _ := setupSyncerLeaseIntegration(t, nil)
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
	for _, stale := range []SyncerLease{
		{Owner: "different-owner", Generation: second.Generation},
		{Owner: second.Owner, Generation: second.Generation + 1},
	} {
		if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, stale); !errors.Is(err, ErrSyncerLeaseLost) {
			t.Fatalf("mismatched acknowledgement error = %v, want ErrSyncerLeaseLost", err)
		}
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

func expireSyncerLeaseForIntegration(t *testing.T, ctx context.Context, collection *mongo.Collection) {
	t.Helper()
	_, err := collection.UpdateOne(ctx, bson.M{"_id": activeSyncerLeaseID}, mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{"expiresAt": bson.M{"$dateSubtract": bson.M{
			"startDate": "$$NOW", "unit": "second", "amount": 1,
		}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readSyncerLeaseForIntegration(t *testing.T, ctx context.Context, collection *mongo.Collection) bson.M {
	t.Helper()
	var row bson.M
	if err := collection.FindOne(ctx, bson.M{"_id": activeSyncerLeaseID}).Decode(&row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSyncerLeaseRebootRecovery(t *testing.T) {
	current := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID}
	oldIdentity := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerOldBootID}
	newOwner := formatSyncerLeaseOwner(43, []byte("0123456789abcdef"), current)
	tests := []struct {
		name       string
		old        syncerLeaseIdentity
		current    syncerLeaseIdentity
		owner      string
		expired    bool
		canRecover bool
	}{
		{name: "earlier boot on same machine", old: oldIdentity, current: current, owner: newOwner, expired: true, canRecover: true},
		{
			name:    "different machine",
			old:     syncerLeaseIdentity{machineID: "22222222222222222222222222222222", bootID: testSyncerOldBootID},
			current: current, owner: newOwner, expired: true,
		},
		{name: "same boot", old: current, current: current, owner: newOwner, expired: true},
		{name: "before expiry", old: oldIdentity, current: current, owner: newOwner},
		{name: "old format owner", current: current, owner: newOwner, expired: true},
		{name: "unavailable local identity", old: oldIdentity, owner: newOwner, expired: true},
		{name: "new owner lacks identity", old: oldIdentity, current: current, owner: testSyncerOldOwner, expired: true},
		{
			name: "new owner differs from local identity", old: oldIdentity, owner: newOwner, expired: true,
			current: syncerLeaseIdentity{machineID: "22222222222222222222222222222222", bootID: testSyncerBootID},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var reads atomic.Int32
			ctx, collection := setupSyncerLeaseIntegration(t, &event.CommandMonitor{
				Started: func(_ context.Context, event *event.CommandStartedEvent) {
					if event.CommandName == "find" {
						reads.Add(1)
					}
				},
			})
			oldOwner := formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), test.old)
			previous, err := AcquireSyncerLease(ctx, oldOwner, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if test.expired {
				expireSyncerLeaseForIntegration(t, ctx, collection)
			}
			before := readSyncerLeaseForIntegration(t, ctx, collection)
			reads.Store(0)
			logCore, logs := observer.New(zap.InfoLevel)
			previousLogger := Logger
			Logger = zap.New(logCore)
			t.Cleanup(func() { Logger = previousLogger })

			next, err := acquireSyncerLease(ctx, test.owner, time.Minute, test.current)
			if reads.Load() != 1 {
				t.Fatalf("recovery snapshot reads = %d, want 1", reads.Load())
			}
			if test.canRecover {
				if err != nil || next.Owner != test.owner || next.Generation != previous.Generation+1 {
					t.Fatalf("recovered lease = %+v, error = %v", next, err)
				}
				if logs.Len() != 1 {
					t.Fatalf("recovery log count = %d, want 1", logs.Len())
				}
				fields := logs.All()[0].ContextMap()
				if fields["owner"] != oldOwner || fields["generation"] != previous.Generation || fields["reason"] != "same machine with a different boot identity" {
					t.Fatalf("recovery log fields = %+v", fields)
				}
				after := readSyncerLeaseForIntegration(t, ctx, collection)
				if after["releasedAt"] != nil || after["releasedGeneration"] != nil {
					t.Fatalf("new generation retained a release acknowledgement: %+v", after)
				}
				if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, previous); !errors.Is(err, ErrSyncerLeaseLost) {
					t.Fatalf("stale acknowledgement error = %v, want ErrSyncerLeaseLost", err)
				}
				return
			}
			if !errors.Is(err, ErrSyncerLeaseHeld) {
				t.Fatalf("recovery refusal = %v, want ErrSyncerLeaseHeld", err)
			}
			if after := readSyncerLeaseForIntegration(t, ctx, collection); !reflect.DeepEqual(before, after) {
				t.Fatalf("refused recovery changed the lease: before=%+v after=%+v", before, after)
			}
			if logs.Len() != 0 {
				t.Fatalf("refused recovery logged %d acknowledgements", logs.Len())
			}
			// The operator acknowledgement continues to accept every owner format
			// and identity once the lease has expired and compute is fenced.
			if !test.expired {
				if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, previous); !errors.Is(err, ErrSyncerLeaseLost) {
					t.Fatalf("unexpired acknowledgement error = %v, want ErrSyncerLeaseLost", err)
				}
				expireSyncerLeaseForIntegration(t, ctx, collection)
			}
			if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, previous); err != nil {
				t.Fatal(err)
			}
			if next, err := AcquireSyncerLease(ctx, test.owner, time.Minute); err != nil || next.Generation != previous.Generation+1 {
				t.Fatalf("acquire after manual acknowledgement = %+v, error = %v", next, err)
			}
		})
	}
}

func TestSyncerLeaseRebootRecoveryWithLocalIdentity(t *testing.T) {
	ctx, collection := setupSyncerLeaseIntegration(t, nil)
	owner, err := NewSyncerLeaseOwner()
	if err != nil {
		t.Fatal(err)
	}
	old := parseSyncerLeaseOwner(owner)
	if !old.known() {
		t.Skip("local machine and boot identity are unavailable")
	}
	old.bootID = testSyncerOldBootID
	if old.bootID == parseSyncerLeaseOwner(owner).bootID {
		old.bootID = testSyncerBootID
	}
	previous, err := AcquireSyncerLease(ctx, formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), old), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expireSyncerLeaseForIntegration(t, ctx, collection)
	next, err := AcquireSyncerLease(ctx, owner, time.Minute)
	if err != nil || next.Owner != owner || next.Generation != previous.Generation+1 {
		t.Fatalf("reboot recovery with local identity = %+v, error = %v", next, err)
	}
}

func TestSyncerLeaseRebootRecoveryErrorsPreserveHeld(t *testing.T) {
	for _, test := range []struct{ name, cancelAfter string }{
		{name: "snapshot read error", cancelAfter: "findAndModify"},
		{name: "acknowledgement update error", cancelAfter: "find"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var armed atomic.Bool
			var cancelAttempt context.CancelFunc
			ctx, collection := setupSyncerLeaseIntegration(t, &event.CommandMonitor{
				Succeeded: func(_ context.Context, event *event.CommandSucceededEvent) {
					if event.CommandName == test.cancelAfter && armed.Swap(false) {
						cancelAttempt()
					}
				},
			})
			current := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID}
			old := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerOldBootID}
			if _, err := AcquireSyncerLease(ctx, formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), old), time.Minute); err != nil {
				t.Fatal(err)
			}
			expireSyncerLeaseForIntegration(t, ctx, collection)
			before := readSyncerLeaseForIntegration(t, ctx, collection)
			attemptCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			cancelAttempt = cancel
			armed.Store(true)
			_, err := acquireSyncerLease(attemptCtx, formatSyncerLeaseOwner(43, []byte("0123456789abcdef"), current), time.Minute, current)
			if armed.Load() || !errors.Is(err, ErrSyncerLeaseHeld) {
				t.Fatalf("recovery error = %v, injection pending = %t", err, armed.Load())
			}
			if after := readSyncerLeaseForIntegration(t, ctx, collection); !reflect.DeepEqual(before, after) {
				t.Fatalf("failed recovery changed the lease: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestSyncerLeaseRebootRecoveryConcurrentAcquirers(t *testing.T) {
	ctx, collection := setupSyncerLeaseIntegration(t, nil)
	current := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID}
	old := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerOldBootID}
	previous, err := AcquireSyncerLease(ctx, formatSyncerLeaseOwner(42, []byte("0123456789abcdef"), old), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expireSyncerLeaseForIntegration(t, ctx, collection)

	const contenders = 8
	type result struct {
		lease SyncerLease
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, contenders)
	for i := 0; i < contenders; i++ {
		go func(pid int) {
			<-start
			owner := formatSyncerLeaseOwner(pid, []byte("0123456789abcdef"), current)
			lease, err := acquireSyncerLease(ctx, owner, time.Minute, current)
			results <- result{lease: lease, err: err}
		}(100 + i)
	}
	close(start)
	winners := 0
	for i := 0; i < contenders; i++ {
		result := <-results
		if result.err == nil {
			winners++
			if result.lease.Generation != previous.Generation+1 {
				t.Errorf("winning generation = %d, want %d", result.lease.Generation, previous.Generation+1)
			}
		} else if !errors.Is(result.err, ErrSyncerLeaseHeld) {
			t.Errorf("competing acquire error = %v, want ErrSyncerLeaseHeld", result.err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent winners = %d, want 1", winners)
	}
	if err := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, previous); !errors.Is(err, ErrSyncerLeaseLost) {
		t.Fatalf("stale acknowledgement error = %v, want ErrSyncerLeaseLost", err)
	}
}
