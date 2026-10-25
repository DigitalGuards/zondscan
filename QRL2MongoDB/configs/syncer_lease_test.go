package configs

import (
	"os"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestSyncerLeaseAcquireFilterRequiresFreshOrCleanlyReleasedLease(t *testing.T) {
	got := syncerLeaseAcquireFilter("owner-a")
	want := bson.M{
		"_id": activeSyncerLeaseID,
		"$or": bson.A{
			bson.M{"owner": bson.M{"$exists": false}},
			bson.M{"$and": bson.A{
				bson.M{"releasedAt": bson.M{"$exists": true}},
				bson.M{"$expr": bson.M{
					"$and": bson.A{
						bson.M{"$lte": bson.A{"$expiresAt", "$$NOW"}},
						bson.M{"$eq": bson.A{"$releasedGeneration", "$generation"}},
					},
				}},
			}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lease filter = %#v, want %#v", got, want)
	}
}

func TestSyncerLeaseUpdatesUseMongoServerTime(t *testing.T) {
	ttl := 2 * time.Minute
	expiry := bson.M{"$dateAdd": bson.M{
		"startDate": "$$NOW",
		"unit":      "millisecond",
		"amount":    int64(120000),
	}}
	wantAcquire := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "owner", Value: "owner-a"},
			{Key: "generation", Value: bson.M{
				"$add": bson.A{bson.M{"$ifNull": bson.A{"$generation", int64(0)}}, int64(1)},
			}},
			{Key: "expiresAt", Value: expiry},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{
			"releasedAt",
			"releasedGeneration",
		}}},
	}
	if got := syncerLeaseAcquireUpdate("owner-a", ttl); !reflect.DeepEqual(got, wantAcquire) {
		t.Fatalf("acquire update = %#v, want %#v", got, wantAcquire)
	}

	wantRenew := mongo.Pipeline{bson.D{{Key: "$set", Value: bson.D{
		{Key: "expiresAt", Value: expiry},
	}}}}
	if got := syncerLeaseRenewUpdate(ttl); !reflect.DeepEqual(got, wantRenew) {
		t.Fatalf("renew update = %#v, want %#v", got, wantRenew)
	}
}

func TestNewSyncerLeaseOwnerIsUniqueAndNonEmpty(t *testing.T) {
	first, err := NewSyncerLeaseOwner()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSyncerLeaseOwner()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || second == "" || first == second {
		t.Fatalf("lease owners = %q and %q", first, second)
	}
	current := readSyncerLeaseIdentity(os.ReadFile)
	if parseSyncerLeaseOwner(first) != current || parseSyncerLeaseOwner(second) != current {
		t.Fatal("generated owners must include the available local identity")
	}
}

func TestCanRecoverSyncerLeaseAfterReboot(t *testing.T) {
	current := syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerBootID}
	previous := SyncerLease{
		Owner:      testSyncerOldOwner + ";machine=" + testSyncerMachineID + ";boot=" + testSyncerOldBootID,
		Generation: 7,
	}
	tests := []struct {
		name     string
		previous SyncerLease
		expired  bool
		current  syncerLeaseIdentity
		want     bool
	}{
		{name: "same machine earlier boot expired", previous: previous, expired: true, current: current, want: true},
		{name: "before expiry", previous: previous, current: current},
		{name: "unknown current identity", previous: previous, expired: true},
		{name: "missing current machine", previous: previous, expired: true, current: syncerLeaseIdentity{bootID: testSyncerBootID}},
		{name: "missing current boot", previous: previous, expired: true, current: syncerLeaseIdentity{machineID: testSyncerMachineID}},
		{
			name: "different machine", previous: previous, expired: true,
			current: syncerLeaseIdentity{machineID: "22222222222222222222222222222222", bootID: testSyncerBootID},
		},
		{
			name: "same boot", previous: previous, expired: true,
			current: syncerLeaseIdentity{machineID: testSyncerMachineID, bootID: testSyncerOldBootID},
		},
		{
			name: "old format", previous: SyncerLease{Owner: testSyncerOldOwner, Generation: 7},
			expired: true, current: current,
		},
		{
			name: "unknown owner", previous: SyncerLease{Owner: "unknown", Generation: 7},
			expired: true, current: current,
		},
		{
			name: "missing old boot", previous: SyncerLease{Owner: testSyncerOldOwner + ";machine=" + testSyncerMachineID, Generation: 7},
			expired: true, current: current,
		},
		{
			name: "missing old machine", previous: SyncerLease{Owner: testSyncerOldOwner + ";boot=" + testSyncerOldBootID, Generation: 7},
			expired: true, current: current,
		},
		{
			name: "invalid generation", previous: SyncerLease{Owner: previous.Owner},
			expired: true, current: current,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canRecoverSyncerLeaseAfterReboot(test.previous, test.expired, test.current); got != test.want {
				t.Fatalf("reboot recovery decision = %t, want %t", got, test.want)
			}
		})
	}
}
