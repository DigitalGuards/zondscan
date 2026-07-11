package db

import "go.mongodb.org/mongo-driver/bson"

// addressOrFilter builds the { $or: [ {from: addr}, {to: addr} ] } filter
// shared by every "rows touching this address" query in this package.
// Callers pass the already-canonicalised address (normalizeAddress); this
// helper only shapes the filter. Every document in the filter is
// single-key, so the bson.M form marshals to the same wire bytes as the
// primitive.D literals it replaced.
func addressOrFilter(address string) bson.M {
	return bson.M{"$or": []bson.M{
		{"from": address},
		{"to": address},
	}}
}

// clampPage normalizes 1-based pagination arguments: page floors at 1, a
// non-positive limit becomes defaultLimit, and when maxLimit > 0 the
// limit is capped at maxLimit (0 means uncapped). Defaults and caps are
// per-endpoint frontend contracts, so every caller passes its own values;
// this helper only removes the repeated clamping boilerplate.
//
// Only for the (page < 1, limit < 1, limit > cap) pattern. Functions with
// different semantics (0-based pages, page == 0 only, limit-only clamps)
// keep their inline logic, changing them would change which inputs are
// accepted.
func clampPage(page, limit, defaultLimit, maxLimit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}
