package db

import (
	"context"
	"log"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// fallbackLogged tracks which collections have already logged the
// metadata-fallback warning, so we surface it once per collection per process
// instead of on every (currently always-0) count call.
var fallbackLogged sync.Map

// countDocumentsResilient returns an accurate document count for list-pagination
// totals.
//
// EstimatedDocumentCount is a near-instant metadata read, but on this
// deployment that metadata reads 0 for every collection: the WiredTiger
// fast-count was reset during the June 2026 infra consolidation (a restore /
// unclean shutdown) and never rebuilt, so a populated collection reports 0
// until a validate() rebuilds it. A 0 total collapses every paginated view to
// "Page X of 1" and disables Next.
//
// So we keep the cheap metadata read on the happy path and fall back to an
// exact CountDocuments only when it reads 0. CountDocuments on an empty
// collection is itself instant, and the callers that matter cache the result
// (routeCache, ~10s), so the exact count runs at most once per window. When the
// metadata is eventually rebuilt this transparently reverts to the fast path.
func countDocumentsResilient(ctx context.Context, coll *mongo.Collection) (int64, error) {
	est, err := coll.EstimatedDocumentCount(ctx)
	if err != nil {
		return 0, err
	}
	if est > 0 {
		return est, nil
	}
	if _, seen := fallbackLogged.LoadOrStore(coll.Name(), true); !seen {
		log.Printf("[count] EstimatedDocumentCount returned 0 for %q on a query; using exact CountDocuments. The WiredTiger fast-count metadata is stale: run validate() on this collection to rebuild it and restore the fast path.", coll.Name())
	}
	return coll.CountDocuments(ctx, primitive.D{})
}
