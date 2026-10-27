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
	ValidatorIndices []string
}

func prepareValidatorActivity(ctx context.Context, source validatorHistorySource, store validatorHistoryStore, epoch int64, validators []models.StateValidator, ejectionBalance uint64) (validatorActivity, *models.SlashingSnapshot, error) {
	changes, snapshot, err := services.ValidatorChanges(epoch, validators, ejectionBalance)
	activity := validatorActivity{Changes: changes}
	if err != nil {
		return activity, nil, err
	}
	stored, err := store.Snapshot(ctx, epoch)
	if err != nil {
		return activity, nil, err
	}
	if stored != nil {
		snapshot.Resolved = stored.Resolved
		snapshot.EvidenceMissing = stored.EvidenceMissing
	}
	resolve := func(older, newer *models.SlashingSnapshot) error {
		if older == nil || newer == nil || newer.Resolved {
			return nil
		}
		changes, err := services.SlashingChanges(*older, *newer)
		if err != nil {
			return err
		}
		resolution := slashingResolution{Epoch: newer.Epoch}
		if len(changes) > 0 {
			events, evidenceErr := source.SlashingEvidence(ctx, newer.Epoch)
			if err := ctx.Err(); err != nil {
				return err
			}
			if evidenceErr != nil && !errors.Is(evidenceErr, models.ErrStructuralSlashingEvidence) {
				return evidenceErr
			}
			resolved, attributionErr := services.ResolveSlashingEvents(events, changes, *newer)
			resolution.EvidenceErr = errors.Join(evidenceErr, attributionErr)
			for _, change := range changes {
				resolution.ValidatorIndices = append(resolution.ValidatorIndices, change.ValidatorIndex)
			}
			activity.Events = append(activity.Events, resolved...)
			activity.Changes = append(activity.Changes, changes...)
		}
		newer.Resolved = true
		newer.EvidenceMissing = resolution.EvidenceErr != nil
		activity.Resolutions = append(activity.Resolutions, resolution)
		return nil
	}
	older := &models.SlashingSnapshot{Epoch: -1}
	if epoch > 0 {
		older, err = store.Snapshot(ctx, epoch-1)
		if err != nil {
			return activity, nil, err
		}
	}
	if err := resolve(older, &snapshot); err != nil {
		return activity, nil, err
	}
	// Descending backfill completes the newer snapshot's comparison here.
	newer, err := store.Snapshot(ctx, epoch+1)
	if err != nil {
		return activity, nil, err
	}
	if err := resolve(&snapshot, newer); err != nil {
		return activity, nil, err
	}
	return activity, &snapshot, nil
}

func (mongoValidatorHistoryStore) Snapshot(ctx context.Context, epoch int64) (*models.SlashingSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var row struct {
		Snapshot *models.SlashingSnapshot `bson:"slashingSnapshot"`
	}
	err := configs.ValidatorHistoryCollections.FindOne(ctx,
		bson.M{"epochInt": epoch, "source": models.BeaconStateHistorySource},
		options.FindOne().SetProjection(bson.M{"slashingSnapshot": 1})).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return row.Snapshot, err
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
