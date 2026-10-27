package db

import (
	"context"
	"errors"
	"time"

	"backendAPI/configs"
	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func activityFilter(q models.ValidatorActivityQuery, events bool) bson.M {
	filter := bson.M{}
	if q.ValidatorIndex != "" {
		field := "validatorIndex"
		if events {
			field = "validatorIndices"
		}
		filter[field] = q.ValidatorIndex
	}
	if q.Kind != "" {
		filter["kind"] = q.Kind
	}
	if q.FromEpoch != nil || q.ToEpoch != nil {
		rangeFilter := bson.M{}
		if q.FromEpoch != nil {
			rangeFilter["$gte"] = *q.FromEpoch
		}
		if q.ToEpoch != nil {
			rangeFilter["$lte"] = *q.ToEpoch
		}
		filter["epoch"] = rangeFilter
	}
	return filter
}

type activityCollection interface {
	CountDocuments(context.Context, interface{}, ...*options.CountOptions) (int64, error)
	Find(context.Context, interface{}, ...*options.FindOptions) (*mongo.Cursor, error)
}

func activityPage[T any](ctx context.Context, collection activityCollection, q models.ValidatorActivityQuery, events bool) ([]T, int64, error) {
	filter := activityFilter(q, events)
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	field := "epoch"
	if events {
		field = "slot"
	}
	find := options.Find().SetSort(bson.D{{Key: field, Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64(q.Page-1) * int64(q.Limit)).SetLimit(int64(q.Limit))
	cursor, err := collection.Find(ctx, filter, find)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	rows := make([]T, 0)
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func validatorActivityBackfillComplete(ctx context.Context) (bool, error) {
	var state struct {
		Version      int   `bson:"version"`
		BackfillNext int64 `bson:"backfillNext"`
	}
	err := configs.SyncStateCollection.FindOne(ctx, bson.M{"_id": "validator_history"}).Decode(&state)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return state.Version >= 2 && state.BackfillNext == -1, err
}

func GetSlashings(ctx context.Context, q models.ValidatorActivityQuery) (*models.SlashingsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	events, total, err := activityPage[models.SlashingEvent](ctx, configs.SlashingEventsCollection, q, true)
	if err != nil {
		return nil, err
	}
	count, err := configs.ValidatorsCollections.CountDocuments(ctx, bson.M{"slashed": true})
	if err != nil {
		return nil, err
	}
	complete, err := validatorActivityBackfillComplete(ctx)
	if err != nil {
		return nil, err
	}
	return &models.SlashingsResponse{Events: events, Total: total, Page: q.Page, Limit: q.Limit, SlashedValidatorCount: count, BackfillComplete: complete}, nil
}

func GetValidatorSetChanges(ctx context.Context, q models.ValidatorActivityQuery) (*models.ValidatorSetChangesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	changes, total, err := activityPage[models.ValidatorSetChange](ctx, configs.ValidatorSetChangesCollection, q, false)
	if err != nil {
		return nil, err
	}
	complete, err := validatorActivityBackfillComplete(ctx)
	if err != nil {
		return nil, err
	}
	return &models.ValidatorSetChangesResponse{Changes: changes, Total: total, Page: q.Page, Limit: q.Limit, BackfillComplete: complete}, nil
}
