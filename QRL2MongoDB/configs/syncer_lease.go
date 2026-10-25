package configs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const activeSyncerLeaseID = "chain-indexer"

var (
	ErrSyncerLeaseHeld = errors.New("chain-indexer lease is active or lacks a clean release acknowledgement")
	ErrSyncerLeaseLost = errors.New("chain-indexer lease was lost or expired")
)

type SyncerLease struct {
	Owner      string    `bson:"owner"`
	Generation int64     `bson:"generation"`
	ExpiresAt  time.Time `bson:"expiresAt"`
}

func AcquireSyncerLease(ctx context.Context, owner string, ttl time.Duration) (SyncerLease, error) {
	return acquireSyncerLease(ctx, owner, ttl, readSyncerLeaseIdentity(os.ReadFile))
}

func acquireSyncerLease(ctx context.Context, owner string, ttl time.Duration, current syncerLeaseIdentity) (SyncerLease, error) {
	var lease SyncerLease
	if SyncerLeasesCollection == nil || owner == "" || ttl < time.Millisecond {
		return lease, fmt.Errorf("invalid syncer lease configuration")
	}
	if err := ensureSyncerLeaseDocument(ctx); err != nil {
		return lease, err
	}
	lease, err := tryAcquireSyncerLease(ctx, owner, ttl)
	if !errors.Is(err, ErrSyncerLeaseHeld) {
		return lease, err
	}

	// Read one snapshot and evaluate expiry using MongoDB's clock. The exact
	// owner/generation acknowledgement rechecks expiry if the row changes.
	var previous struct {
		SyncerLease `bson:",inline"`
		Expired     bool `bson:"expired"`
	}
	readErr := SyncerLeasesCollection.FindOne(ctx,
		bson.M{"_id": activeSyncerLeaseID},
		options.FindOne().SetProjection(bson.M{
			"owner":      1,
			"generation": 1,
			"expiresAt":  1,
			"expired":    bson.M{"$lte": bson.A{"$expiresAt", "$$NOW"}},
		}),
	).Decode(&previous)
	if readErr != nil || parseSyncerLeaseOwner(owner) != current ||
		!canRecoverSyncerLeaseAfterReboot(previous.SyncerLease, previous.Expired, current) {
		return SyncerLease{}, err
	}
	if acknowledgeErr := AcknowledgeExpiredSyncerLeaseAfterComputeFence(ctx, previous.SyncerLease); acknowledgeErr != nil {
		return SyncerLease{}, err
	}
	Logger.Info("Acknowledged expired syncer lease after reboot",
		zap.String("owner", previous.Owner),
		zap.Int64("generation", previous.Generation),
		zap.String("reason", "same machine with a different boot identity"))
	return tryAcquireSyncerLease(ctx, owner, ttl)
}

func canRecoverSyncerLeaseAfterReboot(previous SyncerLease, expired bool, current syncerLeaseIdentity) bool {
	old := parseSyncerLeaseOwner(previous.Owner)
	return expired && previous.Generation > 0 && current.known() && old.known() &&
		old.machineID == current.machineID && old.bootID != current.bootID
}

func tryAcquireSyncerLease(ctx context.Context, owner string, ttl time.Duration) (SyncerLease, error) {
	var lease SyncerLease
	err := SyncerLeasesCollection.FindOneAndUpdate(
		ctx,
		syncerLeaseAcquireFilter(owner),
		syncerLeaseAcquireUpdate(owner, ttl),
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&lease)
	if errors.Is(err, mongo.ErrNoDocuments) || mongo.IsDuplicateKeyError(err) {
		return SyncerLease{}, ErrSyncerLeaseHeld
	}
	if err != nil {
		return SyncerLease{}, err
	}
	return lease, nil
}

// ensureSyncerLeaseDocument creates the fixed lease row before acquisition.
// MongoDB forbids $expr in an upsert predicate, so acquisition itself cannot
// safely combine server-time expiry comparison with SetUpsert. This seed uses
// an _id-only upsert and a server-time pipeline, then acquisition performs a
// non-upsert compare-and-swap against the existing row.
func ensureSyncerLeaseDocument(ctx context.Context) error {
	_, err := SyncerLeasesCollection.UpdateOne(
		ctx,
		bson.M{"_id": activeSyncerLeaseID},
		mongo.Pipeline{bson.D{{Key: "$set", Value: bson.D{
			{Key: "generation", Value: bson.M{
				"$ifNull": bson.A{"$generation", int64(0)},
			}},
			{Key: "expiresAt", Value: bson.M{
				"$ifNull": bson.A{"$expiresAt", bson.M{
					"$dateSubtract": bson.M{
						"startDate": "$$NOW",
						"unit":      "millisecond",
						"amount":    int64(1),
					},
				}},
			}},
		}}}},
		options.Update().SetUpsert(true),
	)
	return err
}

func RenewSyncerLease(ctx context.Context, lease SyncerLease, ttl time.Duration) (SyncerLease, error) {
	if SyncerLeasesCollection == nil || lease.Owner == "" || lease.Generation <= 0 || ttl < time.Millisecond {
		return SyncerLease{}, ErrSyncerLeaseLost
	}
	var renewed SyncerLease
	err := SyncerLeasesCollection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":        activeSyncerLeaseID,
			"owner":      lease.Owner,
			"generation": lease.Generation,
			"$expr": bson.M{
				"$gt": bson.A{"$expiresAt", "$$NOW"},
			},
		},
		syncerLeaseRenewUpdate(ttl),
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&renewed)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return SyncerLease{}, ErrSyncerLeaseLost
	}
	if err != nil {
		return SyncerLease{}, err
	}
	return renewed, nil
}

func ReleaseSyncerLease(ctx context.Context, lease SyncerLease) error {
	if SyncerLeasesCollection == nil || lease.Owner == "" || lease.Generation <= 0 {
		return nil
	}
	result, err := SyncerLeasesCollection.UpdateOne(ctx, bson.M{
		"_id":        activeSyncerLeaseID,
		"owner":      lease.Owner,
		"generation": lease.Generation,
	}, mongo.Pipeline{bson.D{{Key: "$set", Value: bson.D{
		{Key: "expiresAt", Value: "$$NOW"},
		{Key: "releasedAt", Value: "$$NOW"},
		{Key: "releasedGeneration", Value: lease.Generation},
	}}}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return ErrSyncerLeaseLost
	}
	return nil
}

// AcknowledgeExpiredSyncerLeaseAfterComputeFence marks an expired lease as
// cleanly released. Callers must first prove the old process or host cannot
// execute again. The exact owner and generation comparison prevents an
// operator from acknowledging a lease that changed after inspection.
func AcknowledgeExpiredSyncerLeaseAfterComputeFence(
	ctx context.Context,
	expected SyncerLease,
) error {
	if SyncerLeasesCollection == nil || expected.Owner == "" || expected.Generation <= 0 {
		return ErrSyncerLeaseLost
	}
	result, err := SyncerLeasesCollection.UpdateOne(
		ctx,
		bson.M{
			"_id":        activeSyncerLeaseID,
			"owner":      expected.Owner,
			"generation": expected.Generation,
			"$expr": bson.M{
				"$lte": bson.A{"$expiresAt", "$$NOW"},
			},
		},
		mongo.Pipeline{bson.D{{Key: "$set", Value: bson.D{
			{Key: "releasedAt", Value: "$$NOW"},
			{Key: "releasedGeneration", Value: expected.Generation},
		}}}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return ErrSyncerLeaseLost
	}
	return nil
}

// syncerLeaseAcquireFilter permits a new generation only for a fresh lease
// row or after the prior owner recorded a clean release. Expiry alone is not
// sufficient: a suspended process can resume after its TTL and continue a
// Mongo mutation that began while it still owned the lease. Requiring clean
// release prevents automatic split-brain takeover. Recovery requires proof
// that the old process cannot execute: a different boot on this machine or
// an explicit operator acknowledgement after compute fencing.
func syncerLeaseAcquireFilter(_ string) bson.M {
	return bson.M{
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
}

func syncerLeaseAcquireUpdate(owner string, ttl time.Duration) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "owner", Value: owner},
			{Key: "generation", Value: bson.M{
				"$add": bson.A{bson.M{"$ifNull": bson.A{"$generation", int64(0)}}, int64(1)},
			}},
			{Key: "expiresAt", Value: leaseExpiryExpression(ttl)},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{
			"releasedAt",
			"releasedGeneration",
		}}},
	}
}

func syncerLeaseRenewUpdate(ttl time.Duration) mongo.Pipeline {
	return mongo.Pipeline{bson.D{{Key: "$set", Value: bson.D{
		{Key: "expiresAt", Value: leaseExpiryExpression(ttl)},
	}}}}
}

func leaseExpiryExpression(ttl time.Duration) bson.M {
	return bson.M{"$dateAdd": bson.M{
		"startDate": "$$NOW",
		"unit":      "millisecond",
		"amount":    ttl.Milliseconds(),
	}}
}
