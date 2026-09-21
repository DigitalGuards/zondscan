package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type documentCounter interface {
	CountDocuments(context.Context, interface{}, ...*options.CountOptions) (int64, error)
}

// countDocumentsResilient returns an accurate document count for list-pagination
// totals. Metadata estimates can be stale even when they are positive. The _id_
// hint makes this unfiltered count scan the index while retaining the caller's
// deadline. Its cost grows with the collection; route caches remain caller-owned.
func countDocumentsResilient(ctx context.Context, coll documentCounter) (int64, error) {
	return coll.CountDocuments(ctx, primitive.D{}, options.Count().SetHint("_id_"))
}
