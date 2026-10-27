package synchroniser

import (
	"context"
	"errors"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const validatorSetChangesCollection = "validatorSetChanges"
const slashingEventsCollection = "slashingEvents"

type validatorActivity struct {
	Changes     []models.ValidatorSetChange
	Events      []models.SlashingEvent
	Resolutions []slashingResolution
}

type slashingResolution struct {
	Epoch            int64
	EvidenceErr      error
	EvidenceRetryAt  int64
	ValidatorIndices []string
}

func prepareValidatorActivity(ctx context.Context, source validatorHistorySource, store validatorHistoryStore, epoch int64, validators []models.StateValidator, ejectionBalance uint64) (validatorActivity, *models.SlashingSnapshot, error) {
	changes, snapshot, err := services.ValidatorChanges(epoch, validators, ejectionBalance)
	activity := validatorActivity{Changes: changes}
	if err != nil {
		return activity, nil, err
	}
	snapshots, err := store.Snapshots(ctx, []int64{epoch - 1, epoch, epoch + 1})
	if err != nil {
		return activity, nil, err
	}
	if stored := snapshots[epoch]; stored != nil {
		snapshot.Resolved = stored.Resolved
		snapshot.EvidenceMissing = stored.EvidenceMissing
		snapshot.EvidenceRetryAt = stored.EvidenceRetryAt
	}
	resolve := func(older, newer *models.SlashingSnapshot) error {
		if older == nil || newer == nil || newer.Resolved {
			return nil
		}
		changes, err := services.SlashingChanges(*older, *newer, validators)
		if err != nil {
			return err
		}
		resolution := slashingResolution{Epoch: newer.Epoch}
		if len(changes) > 0 {
			events, evidenceErr := source.SlashingEvidence(ctx, newer.Epoch)
			if err := ctx.Err(); err != nil {
				return err
			}
			resolved, attributionErr := services.ResolveSlashingEvents(events, changes, *newer)
			resolution.EvidenceErr = errors.Join(evidenceErr, attributionErr)
			if resolution.EvidenceErr != nil {
				resolution.EvidenceRetryAt = time.Now().Add(15 * time.Minute).Unix()
			}
			for _, change := range changes {
				resolution.ValidatorIndices = append(resolution.ValidatorIndices, change.ValidatorIndex)
			}
			activity.Events = append(activity.Events, resolved...)
			activity.Changes = append(activity.Changes, changes...)
		}
		newer.Resolved = true
		newer.EvidenceMissing = resolution.EvidenceErr != nil
		newer.EvidenceRetryAt = resolution.EvidenceRetryAt
		activity.Resolutions = append(activity.Resolutions, resolution)
		return nil
	}
	older := &models.SlashingSnapshot{Epoch: -1}
	if epoch > 0 {
		older = snapshots[epoch-1]
	}
	if err := resolve(older, &snapshot); err != nil {
		return activity, nil, err
	}
	// Descending backfill completes the newer snapshot's comparison here.
	if err := resolve(&snapshot, snapshots[epoch+1]); err != nil {
		return activity, nil, err
	}
	return activity, &snapshot, nil
}

func (mongoValidatorHistoryStore) Snapshots(ctx context.Context, epochs []int64) (map[int64]*models.SlashingSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cursor, err := configs.ValidatorHistoryCollections.Find(ctx,
		bson.M{"epochInt": bson.M{"$in": epochs}, "source": models.BeaconStateHistorySource},
		options.Find().SetProjection(bson.M{"slashingSnapshot": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	snapshots := make(map[int64]*models.SlashingSnapshot, len(epochs))
	for cursor.Next(ctx) {
		var row struct {
			Snapshot *models.SlashingSnapshot `bson:"slashingSnapshot"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		if row.Snapshot != nil {
			snapshots[row.Snapshot.Epoch] = row.Snapshot
		}
	}
	return snapshots, cursor.Err()
}

// Each pass repairs at most one due epoch. Its requests share the beacon client
// and its writes use the same lease as the history walker.
func repairSlashingEvidence(ctx context.Context, source validatorHistorySource, store validatorHistoryStore) error {
	snapshot, err := store.PendingEvidence(ctx, time.Now().Unix())
	if err != nil || snapshot == nil {
		return err
	}
	changes, evidenceErr := store.SlashedChanges(ctx, snapshot.Epoch)
	var resolved []models.SlashingEvent
	if evidenceErr == nil {
		if len(changes) == 0 {
			evidenceErr = errors.New("missing stored slashed changes")
		} else {
			events, fetchErr := source.SlashingEvidence(ctx, snapshot.Epoch)
			var attributionErr error
			resolved, attributionErr = services.ResolveSlashingEvents(events, changes, *snapshot)
			evidenceErr = errors.Join(fetchErr, attributionErr)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	resolution := slashingResolution{Epoch: snapshot.Epoch, EvidenceErr: evidenceErr}
	if evidenceErr != nil {
		resolution.EvidenceRetryAt = time.Now().Add(time.Hour).Unix()
		resolved = nil
	}
	return store.SaveEvidenceRepair(ctx, resolution, resolved)
}

func (mongoValidatorHistoryStore) PendingEvidence(ctx context.Context, now int64) (*models.SlashingSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var row struct {
		Snapshot *models.SlashingSnapshot `bson:"slashingSnapshot"`
	}
	err := configs.ValidatorHistoryCollections.FindOne(ctx,
		bson.M{"source": models.BeaconStateHistorySource, "slashingSnapshot.evidenceMissing": true,
			"slashingSnapshot.evidenceRetryAt": bson.M{"$lte": now}},
		options.FindOne().SetSort(bson.D{{Key: "epochInt", Value: 1}}).
			SetProjection(bson.M{"slashingSnapshot": 1})).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return row.Snapshot, err
}

func (mongoValidatorHistoryStore) SlashedChanges(ctx context.Context, epoch int64) ([]models.ValidatorSetChange, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cursor, err := configs.GetCollection(configs.DB, validatorSetChangesCollection).Find(ctx, bson.M{"epoch": epoch, "kind": "slashed"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var changes []models.ValidatorSetChange
	err = cursor.All(ctx, &changes)
	return changes, err
}

func (mongoValidatorHistoryStore) SaveEvidenceRepair(ctx context.Context, resolution slashingResolution, events []models.SlashingEvent) error {
	return slotAssignmentMutation(ctx, func(ctx context.Context) error {
		if err := storeValidatorActivity(ctx, validatorActivity{Events: events}); err != nil {
			return err
		}
		return storeSlashingResolution(ctx, resolution)
	})
}

func storeSlashingResolution(ctx context.Context, resolution slashingResolution) error {
	update := bson.M{"$set": bson.M{"slashingSnapshot.resolved": true}}
	if resolution.EvidenceErr != nil {
		update["$set"] = bson.M{"slashingSnapshot.resolved": true,
			"slashingSnapshot.evidenceMissing": true, "slashingSnapshot.evidenceRetryAt": resolution.EvidenceRetryAt}
	} else {
		update["$unset"] = bson.M{"slashingSnapshot.evidenceMissing": "", "slashingSnapshot.evidenceRetryAt": ""}
	}
	_, err := configs.ValidatorHistoryCollections.UpdateOne(ctx, bson.M{"epochInt": resolution.Epoch}, update)
	return err
}

func initializeValidatorActivity(ctx context.Context) error {
	for _, spec := range []struct {
		name    string
		indexes []mongo.IndexModel
	}{
		{validatorSetChangesCollection, []mongo.IndexModel{
			{Keys: bson.D{{Key: "epoch", Value: -1}, {Key: "_id", Value: -1}}},
			{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "epoch", Value: -1}, {Key: "_id", Value: -1}}},
			{Keys: bson.D{{Key: "validatorIndex", Value: 1}, {Key: "epoch", Value: -1}, {Key: "_id", Value: -1}}},
		}},
		{slashingEventsCollection, []mongo.IndexModel{
			{Keys: bson.D{{Key: "slot", Value: -1}, {Key: "_id", Value: -1}}},
			{Keys: bson.D{{Key: "validatorIndices", Value: 1}, {Key: "slot", Value: -1}, {Key: "_id", Value: -1}}},
		}},
	} {
		if _, err := configs.GetCollection(configs.DB, spec.name).Indexes().CreateMany(ctx, spec.indexes); err != nil {
			return err
		}
	}
	return nil
}

func storeValidatorActivity(ctx context.Context, activity validatorActivity) error {
	changes := make([]mongo.WriteModel, 0, len(activity.Changes))
	for _, row := range activity.Changes {
		changes = append(changes, mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": row.ID}).SetReplacement(row).SetUpsert(true))
	}
	if len(changes) > 0 {
		if _, err := configs.GetCollection(configs.DB, validatorSetChangesCollection).BulkWrite(ctx, changes); err != nil {
			return err
		}
	}
	events := make([]mongo.WriteModel, 0, len(activity.Events))
	for _, row := range activity.Events {
		events = append(events, mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": row.ID}).SetReplacement(row).SetUpsert(true))
	}
	if len(events) > 0 {
		if _, err := configs.GetCollection(configs.DB, slashingEventsCollection).BulkWrite(ctx, events); err != nil {
			return err
		}
	}
	return nil
}
