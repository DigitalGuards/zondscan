package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"backendAPI/configs"
	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrAIExplanationLeaseBusy   = errors.New("AI explanation generation lease is active")
	ErrAIExplanationCacheFilled = errors.New("AI explanation cache was filled concurrently")
	ErrAIProviderDailyBudget    = errors.New("AI provider daily call budget reached")
)

// AIExplanationLeaseBusyError reports how long the current generation lease
// remains active. Callers expose this as Retry-After without revealing lease
// identity or verification metadata.
type AIExplanationLeaseBusyError struct {
	RetryAfterDuration time.Duration
}

func (e *AIExplanationLeaseBusyError) Error() string {
	return ErrAIExplanationLeaseBusy.Error()
}

func (e *AIExplanationLeaseBusyError) Unwrap() error {
	return ErrAIExplanationLeaseBusy
}

// AIProviderDailyBudgetError reports the time remaining until the next UTC
// daily budget window.
type AIProviderDailyBudgetError struct {
	RetryAfterDuration time.Duration
}

func (e *AIProviderDailyBudgetError) Error() string {
	return ErrAIProviderDailyBudget.Error()
}

func (e *AIProviderDailyBudgetError) Unwrap() error {
	return ErrAIProviderDailyBudget
}

// AcquireAIExplanationLease atomically acquires the single-flight lease for
// one exact source bundle and verification generation.
func AcquireAIExplanationLease(
	ctx context.Context,
	address string,
	expected models.ContractInfo,
	sourceDigest string,
	ttl time.Duration,
	requireCacheMiss bool,
) (models.AIExplanationLease, error) {
	if ttl <= 0 {
		return models.AIExplanationLease{}, fmt.Errorf("AI explanation lease TTL must be positive")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return models.AIExplanationLease{}, fmt.Errorf("create AI explanation lease token: %w", err)
	}
	return acquireAIExplanationLeaseAt(
		ctx,
		address,
		expected,
		sourceDigest,
		ttl,
		time.Now().UTC(),
		hex.EncodeToString(tokenBytes),
		requireCacheMiss,
	)
}

func acquireAIExplanationLeaseAt(
	ctx context.Context,
	address string,
	expected models.ContractInfo,
	sourceDigest string,
	ttl time.Duration,
	now time.Time,
	token string,
	requireCacheMiss bool,
) (models.AIExplanationLease, error) {
	if sourceDigest == "" || expected.VerifiedAt == "" || token == "" {
		return models.AIExplanationLease{}, ErrSourceBundleChanged
	}
	lease := models.AIExplanationLease{
		Token:        token,
		SourceDigest: sourceDigest,
		VerifiedAt:   expected.VerifiedAt,
		AcquiredAt:   now,
		ExpiresAt:    now.Add(ttl),
	}
	normalizedAddr := normalizeAddress(address)
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := configs.ContractInfoCollection.UpdateOne(
		operationCtx,
		explanationLeaseAcquisitionFilter(
			normalizedAddr,
			expected,
			sourceDigest,
			now,
			requireCacheMiss,
		),
		bson.M{"$set": bson.M{"aiExplanationLease": lease}},
	)
	if err != nil {
		return models.AIExplanationLease{}, err
	}
	if res.MatchedCount == 1 {
		return lease, nil
	}

	var current struct {
		Lease                     *models.AIExplanationLease `bson:"aiExplanationLease"`
		AIExplanation             string                     `bson:"aiExplanation"`
		AIExplanationSourceDigest string                     `bson:"aiExplanationSourceDigest"`
	}
	err = configs.ContractInfoCollection.FindOne(
		operationCtx,
		explanationSourceSnapshotFilter(normalizedAddr, expected, sourceDigest),
		options.FindOne().SetProjection(bson.M{
			"aiExplanationLease":        1,
			"aiExplanation":             1,
			"aiExplanationSourceDigest": 1,
		}),
	).Decode(&current)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.AIExplanationLease{}, ErrSourceBundleChanged
	}
	if err != nil {
		return models.AIExplanationLease{}, err
	}
	if requireCacheMiss && current.AIExplanation != "" &&
		current.AIExplanationSourceDigest == sourceDigest {
		return models.AIExplanationLease{}, ErrAIExplanationCacheFilled
	}
	retryAfter := time.Second
	if current.Lease != nil && current.Lease.ExpiresAt.After(now) {
		retryAfter = current.Lease.ExpiresAt.Sub(now)
	}
	return models.AIExplanationLease{}, &AIExplanationLeaseBusyError{
		RetryAfterDuration: retryAfter,
	}
}

func explanationLeaseAcquisitionFilter(
	normalizedAddr string,
	expected models.ContractInfo,
	sourceDigest string,
	now time.Time,
	requireCacheMiss bool,
) bson.M {
	filter := explanationSourceSnapshotFilter(normalizedAddr, expected, sourceDigest)
	leaseAvailable := bson.A{
		bson.M{"aiExplanationLease": bson.M{"$exists": false}},
		bson.M{"aiExplanationLease": nil},
		bson.M{"aiExplanationLease.expiresAt": bson.M{"$exists": false}},
		bson.M{"aiExplanationLease.expiresAt": bson.M{"$lte": now}},
	}
	if !requireCacheMiss {
		filter["$or"] = leaseAvailable
		return filter
	}
	filter["$and"] = bson.A{
		bson.M{"$or": leaseAvailable},
		bson.M{"$or": bson.A{
			bson.M{"aiExplanation": bson.M{"$exists": false}},
			bson.M{"aiExplanation": ""},
			bson.M{"aiExplanationSourceDigest": bson.M{"$ne": sourceDigest}},
		}},
	}
	return filter
}

// CooldownAIExplanationLease keeps a paid-call lease active after a provider
// or cache failure. $max prevents a late failure from shortening an existing
// cooldown.
func CooldownAIExplanationLease(
	ctx context.Context,
	address string,
	lease models.AIExplanationLease,
	cooldown time.Duration,
) error {
	if cooldown <= 0 {
		return fmt.Errorf("AI explanation cooldown must be positive")
	}
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := configs.ContractInfoCollection.UpdateOne(
		operationCtx,
		explanationLeaseIdentityFilter(normalizeAddress(address), lease),
		bson.M{"$max": bson.M{
			"aiExplanationLease.expiresAt": time.Now().UTC().Add(cooldown),
		}},
	)
	return err
}

// ReleaseAIExplanationLease clears a lease when no paid provider call was
// attempted, such as a regeneration-cap or daily-budget rejection.
func ReleaseAIExplanationLease(
	ctx context.Context,
	address string,
	lease models.AIExplanationLease,
) error {
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := configs.ContractInfoCollection.UpdateOne(
		operationCtx,
		explanationLeaseIdentityFilter(normalizeAddress(address), lease),
		bson.M{"$unset": bson.M{"aiExplanationLease": ""}},
	)
	return err
}

func explanationLeaseIdentityFilter(
	normalizedAddr string,
	lease models.AIExplanationLease,
) bson.M {
	return bson.M{
		"address":                         normalizedAddr,
		"aiExplanationLease.token":        lease.Token,
		"aiExplanationLease.sourceDigest": lease.SourceDigest,
		"aiExplanationLease.verifiedAt":   lease.VerifiedAt,
	}
}

// ReserveAIProviderCall atomically consumes one slot in the UTC-day provider
// budget. The first reservation also seals the configured limit for that day,
// so mismatched backend instances fail closed. A capped or mismatched document
// causes the upsert to collide on _id, which is mapped to the typed budget
// error. MongoDB's unique _id index makes the cap safe across all processes.
func ReserveAIProviderCall(ctx context.Context, limit int, now time.Time) error {
	if limit <= 0 {
		return fmt.Errorf("AI provider daily call limit must be positive")
	}
	now = now.UTC()
	dayStart, nextDay := utcProviderBudgetWindow(now)
	documentID := providerBudgetDocumentID(dayStart)
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := providerBudgetReservationFilter(documentID, limit)
	update := bson.M{
		"$inc": bson.M{"count": 1},
		"$setOnInsert": bson.M{
			"day":       dayStart.Format("2006-01-02"),
			"limit":     limit,
			"createdAt": now,
			"expiresAt": nextDay.AddDate(0, 0, 8),
		},
	}
	_, err := configs.ContractExplainUsageCollection.UpdateOne(
		operationCtx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)
	if mongo.IsDuplicateKeyError(err) {
		// Two first callers can race to create a new UTC-day document. Retry
		// once without upsert so a caller does not receive a false cap while
		// the newly inserted document still has capacity.
		res, retryErr := configs.ContractExplainUsageCollection.UpdateOne(
			operationCtx,
			filter,
			update,
		)
		if retryErr != nil {
			return retryErr
		}
		if res.MatchedCount == 1 {
			return nil
		}
		return &AIProviderDailyBudgetError{RetryAfterDuration: nextDay.Sub(now)}
	}
	return err
}

func utcProviderBudgetWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 0, 1)
}

func providerBudgetDocumentID(dayStart time.Time) string {
	return "provider-calls:" + dayStart.UTC().Format("2006-01-02")
}

func providerBudgetReservationFilter(documentID string, limit int) bson.M {
	return bson.M{
		"_id":   documentID,
		"limit": limit,
		"count": bson.M{"$lt": limit},
	}
}
