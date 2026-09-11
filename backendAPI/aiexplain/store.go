package aiexplain

import (
	"context"
	"time"

	"backendAPI/db"
	"backendAPI/models"
)

type modelGenerator interface {
	Generate(context.Context, string, string) (string, string, error)
}

type generationStore interface {
	ReturnContractCode(string) (models.ContractInfo, error)
	AcquireLease(
		context.Context,
		string,
		models.ContractInfo,
		string,
		time.Duration,
		bool,
	) (models.AIExplanationLease, error)
	ReserveRegenSlot(string, int, time.Duration) error
	ReserveProviderCall(context.Context, int, time.Time) error
	SaveExplanation(
		context.Context,
		string,
		string,
		string,
		string,
		string,
		models.AIExplanationLease,
		models.ContractInfo,
	) error
	CooldownLease(context.Context, string, models.AIExplanationLease, time.Duration) error
	ReleaseLease(context.Context, string, models.AIExplanationLease) error
}

type mongoGenerationStore struct{}

func (mongoGenerationStore) ReturnContractCode(address string) (models.ContractInfo, error) {
	return db.ReturnContractCode(address)
}

func (mongoGenerationStore) AcquireLease(
	ctx context.Context,
	address string,
	expected models.ContractInfo,
	sourceDigest string,
	ttl time.Duration,
	requireCacheMiss bool,
) (models.AIExplanationLease, error) {
	return db.AcquireAIExplanationLease(
		ctx,
		address,
		expected,
		sourceDigest,
		ttl,
		requireCacheMiss,
	)
}

func (mongoGenerationStore) ReserveRegenSlot(
	address string,
	limit int,
	window time.Duration,
) error {
	return db.ReserveAIRegenSlot(address, limit, window)
}

func (mongoGenerationStore) ReserveProviderCall(
	ctx context.Context,
	limit int,
	now time.Time,
) error {
	return db.ReserveAIProviderCall(ctx, limit, now)
}

func (mongoGenerationStore) SaveExplanation(
	ctx context.Context,
	address,
	explanation,
	model,
	generatedAt,
	sourceDigest string,
	lease models.AIExplanationLease,
	expected models.ContractInfo,
) error {
	return db.SaveContractExplanation(
		ctx,
		address,
		explanation,
		model,
		generatedAt,
		sourceDigest,
		lease,
		expected,
	)
}

func (mongoGenerationStore) CooldownLease(
	ctx context.Context,
	address string,
	lease models.AIExplanationLease,
	cooldown time.Duration,
) error {
	return db.CooldownAIExplanationLease(ctx, address, lease, cooldown)
}

func (mongoGenerationStore) ReleaseLease(
	ctx context.Context,
	address string,
	lease models.AIExplanationLease,
) error {
	return db.ReleaseAIExplanationLease(ctx, address, lease)
}
