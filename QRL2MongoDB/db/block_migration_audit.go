package db

import (
	"QRL2MongoDB/configs"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AuditStoredCanonicalBlockGaps validates every stored block identity and
// parent link while allowing legacy markerless and pending rows. It returns
// missing heights so the offline migration can merge gap repair and companion
// replay into one ascending work stream.
func AuditStoredCanonicalBlockGaps() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cursor, err := configs.BlocksCollections.Find(
		ctx,
		bson.M{},
		options.Find().
			SetProjection(bson.M{
				"blockNumberInt":    1,
				"result.number":     1,
				"result.hash":       1,
				"result.parenthash": 1,
				"_id":               0,
			}).
			SetSort(bson.D{{Key: "blockNumberInt", Value: 1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	audit := &canonicalChainAudit{previousHeight: -1}
	for cursor.Next(ctx) {
		var row canonicalBlockAuditRow
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		// This inventory pass validates stored identity and linkage. Durable
		// completion is enforced by the final migration audit.
		row.IngestionState = BlockIngestionComplete
		if err := audit.add(row); err != nil {
			return nil, err
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	syncBlock, err := GetLastKnownBlockNumberStrict()
	if errors.Is(err, ErrSyncStateNotFound) {
		if audit.started {
			return nil, fmt.Errorf("%w for a nonempty block collection", ErrSyncStateNotFound)
		}
		return audit.finish()
	}
	if err != nil {
		return nil, err
	}
	syncHeight, err := parseStoredBlockNumber(syncBlock)
	if err != nil {
		return nil, fmt.Errorf("parse durable sync cursor for stored gap audit: %w", err)
	}
	return audit.finishThrough(syncHeight)
}
