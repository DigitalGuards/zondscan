package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/validation"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// UpdateContractMetadata writes the result of a SUCCESSFUL collection-level
// metadata fetch. The fetcher service is the only caller; the classifier
// path (StoreContract) deliberately doesn't touch the resolved fields. This
// separation lets a transient classification blip retain the resolved
// metadata while still allowing the fetcher to record a new fetch attempt's
// outcome without racing the classifier.
//
// Empty `uri` / `name` / `description` / `image` / `externalURL` arguments
// PRESERVE the existing database values (last-good state). `uri` is written
// when the fetcher re-probed contractURI() on a TTL refresh and got a fresh
// value, so an on-chain setContractURI propagates. `metadataFetchedAt` is
// stamped, `metadataFetchError` is cleared, and the retry-scheduling fields
// are removed so the row leaves the retry track.
//
// The caller-supplied ctx scopes both timeout and cancellation. The fetcher
// passes the shutdown-aware ctx so a pm2 stop interrupts an in-flight write
// instead of leaking a 10s context.Background goroutine.
func UpdateContractMetadata(ctx context.Context, address, uri, name, description, image, externalURL, fetchedAt string) error {
	address = validation.ConvertToQAddress(address)

	set := bson.M{
		"metadataFetchedAt":  fetchedAt,
		"metadataFetchError": "",
		"updatedAt":          time.Now().UTC().Format(time.RFC3339),
	}
	// Only overwrite content fields when we have new content, preserving
	// the last-good state.
	if uri != "" {
		set["metadataURI"] = uri
	}
	if name != "" {
		set["metadataName"] = name
	}
	if description != "" {
		set["metadataDescription"] = description
	}
	if image != "" {
		set["metadataImage"] = image
	}
	if externalURL != "" {
		set["metadataExternalURL"] = externalURL
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := configs.GetContractsCollection().UpdateOne(
		ctx,
		bson.M{"address": address},
		bson.M{
			"$set":   set,
			"$unset": bson.M{"metadataRetryCount": "", "metadataNextRetryAt": ""},
		},
	)
	if err != nil {
		configs.Logger.Error("Failed to update contract metadata",
			zap.String("address", address),
			zap.Error(err))
		return err
	}
	return nil
}

// MarkContractMetadataFetchFailed records a failed collection-level fetch:
// the error reason plus the retry-scheduling fields computed by the caller.
// Content fields and metadataFetchedAt are untouched (preserve last-good).
func MarkContractMetadataFetchFailed(ctx context.Context, address, reason string, retryCount int, nextRetryAt string) error {
	address = validation.ConvertToQAddress(address)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := configs.GetContractsCollection().UpdateOne(
		ctx,
		bson.M{"address": address},
		bson.M{"$set": bson.M{
			"metadataFetchError":  reason,
			"metadataRetryCount":  retryCount,
			"metadataNextRetryAt": nextRetryAt,
			"updatedAt":           time.Now().UTC().Format(time.RFC3339),
		}},
	)
	if err != nil {
		configs.Logger.Error("Failed to record contract metadata fetch failure",
			zap.String("address", address),
			zap.Error(err))
		return err
	}
	return nil
}

// GetContractsAwaitingMetadata returns up to `limit` NFT contracts that have
// a populated MetadataURI and are due for a collection-level metadata fetch,
// drawn from the same three tracks as GetTokensAwaitingMetadata and composed
// with the same guaranteed per-track shares (composeBatch):
//
//  1. fresh rows that have never been attempted,
//  2. errored rows whose exponential-backoff deadline passed,
//  3. previously successful rows older than refreshTTL (skipped when
//     refreshTTL <= 0).
//
// Filter note: MetadataURI must EXIST and be non-empty. The naked `$ne: ""`
// form matches docs where the field is missing entirely (Mongo treats
// missing fields as null and null != ""), which would pull in every
// unclassified NFT and trigger an "empty URI" loop.
func GetContractsAwaitingMetadata(ctx context.Context, limit int, now time.Time, refreshTTL time.Duration) ([]models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := configs.GetContractsCollection()
	nowStr := now.UTC().Format(time.RFC3339)
	base := bson.M{
		"metadataURI":   bson.M{"$exists": true, "$ne": ""},
		"tokenStandard": bson.M{"$in": []string{"ERC-721", "ERC-1155"}},
	}
	withBase := func(extra ...bson.M) bson.M {
		f := bson.M{"$and": extra}
		for k, v := range base {
			f[k] = v
		}
		return f
	}

	fresh, err := findLimited[models.ContractInfo](ctx, collection, withBase(
		missingOrEmpty("metadataFetchedAt"),
		missingOrEmpty("metadataFetchError"),
	), limit)
	if err != nil {
		return nil, err
	}

	retryable, err := findLimited[models.ContractInfo](ctx, collection,
		withBase(dueForRetry("metadataFetchError", "metadataNextRetryAt", nowStr)), limit)
	if err != nil {
		return nil, err
	}

	var stale []models.ContractInfo
	if refreshTTL > 0 {
		cutoff := now.UTC().Add(-refreshTTL).Format(time.RFC3339)
		stale, err = findLimited[models.ContractInfo](ctx, collection, withBase(
			bson.M{"metadataFetchedAt": bson.M{"$gt": "", "$lte": cutoff}},
			missingOrEmpty("metadataFetchError"),
		), limit)
		if err != nil {
			return nil, err
		}
	}

	return composeBatch(limit, fresh, retryable, stale), nil
}
