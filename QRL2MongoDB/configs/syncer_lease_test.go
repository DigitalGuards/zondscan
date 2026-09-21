package configs

import (
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
}
