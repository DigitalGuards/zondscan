package synchroniser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const validatorHistoryCursorID = "validator_history"
const validatorHistoryBatchSize = 16
const validatorHistoryVersion = 2

// Two descending ranges keep recent finalized epochs ahead of the old backlog.
// -1 marks a finished range. NewestScheduled covers the original backlog and
// each completed catch-up range; BackfillNext retains unfinished older work.
type validatorHistoryCursor struct {
	Version         int   `bson:"version"`
	BackfillNext    int64 `bson:"backfillNext"`
	NewestScheduled int64 `bson:"newestScheduled"`
	CatchupNext     int64 `bson:"catchupNext"`
	CatchupTarget   int64 `bson:"catchupTarget"`
}

type validatorHistorySource interface {
	FinalizedEpoch(context.Context) (int64, error)
	ValidatorsAtSlot(context.Context, int64) ([]models.StateValidator, error)
	SlashingEvidence(context.Context, int64) ([]models.SlashingEvent, error)
}

type validatorHistoryStore interface {
	Load(context.Context) (validatorHistoryCursor, bool, error)
	Snapshots(context.Context, []int64) (map[int64]*models.SlashingSnapshot, error)
	Save(context.Context, validatorHistoryCursor, *models.ValidatorHistoryRecord, validatorActivity) error
	PendingEvidence(context.Context, int64) (*models.SlashingSnapshot, error)
	SlashedChanges(context.Context, int64) ([]models.ValidatorSetChange, error)
	SaveEvidenceRepair(context.Context, slashingResolution, []models.SlashingEvent) error
}

func walkValidatorHistory(ctx context.Context, source validatorHistorySource, store validatorHistoryStore, timing models.ChainTiming, ejectionBalance uint64) (bool, error) {
	if err := timing.Validate(); err != nil {
		return false, err
	}
	state, exists, err := store.Load(ctx)
	if err != nil {
		return false, err
	}
	finalized, err := source.FinalizedEpoch(ctx)
	if err != nil {
		return false, err
	}
	if _, _, err := timing.EpochSlots(finalized); err != nil {
		return false, err
	}
	if state.Version > validatorHistoryVersion {
		return false, errors.New("unsupported validator history cursor version")
	}
	if !exists || state.Version < validatorHistoryVersion {
		state = validatorHistoryCursor{Version: validatorHistoryVersion, BackfillNext: finalized, NewestScheduled: finalized, CatchupNext: -1, CatchupTarget: -1}
		if err := store.Save(ctx, state, nil, validatorActivity{}); err != nil {
			return false, err
		}
	}
	if state.BackfillNext < -1 || state.BackfillNext > state.NewestScheduled || state.NewestScheduled > finalized || state.CatchupNext < -1 ||
		(state.CatchupNext >= 0 && (state.CatchupNext <= state.NewestScheduled || state.CatchupNext > state.CatchupTarget || state.CatchupTarget > finalized)) {
		return false, errors.New("invalid validator history cursor or regressed finality")
	}
	// Repair failures leave history progress available. Cancellation still stops
	// the pass before any further writes.
	_ = repairSlashingEvidence(ctx, source, store)
	for count := 0; count < validatorHistoryBatchSize; count++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if state.CatchupNext == -1 && finalized > state.NewestScheduled {
			state.CatchupNext, state.CatchupTarget = finalized, finalized
		}
		epoch := state.BackfillNext
		catchingUp := state.CatchupNext >= 0
		if catchingUp {
			epoch = state.CatchupNext
		}
		if epoch < 0 {
			return false, nil
		}
		slot, _, err := timing.EpochSlots(epoch)
		if err != nil {
			return false, err
		}
		validators, err := source.ValidatorsAtSlot(ctx, slot)
		if err != nil {
			return false, fmt.Errorf("validator history epoch %d: %w", epoch, err)
		}
		record, err := services.BuildValidatorHistory(timing, epoch, validators)
		if err != nil {
			return false, fmt.Errorf("validator history epoch %d: %w", epoch, err)
		}
		activity, snapshot, err := prepareValidatorActivity(ctx, source, store, epoch, validators, ejectionBalance)
		if err != nil {
			return false, fmt.Errorf("validator activity epoch %d: %w", epoch, err)
		}
		record.SlashingSnapshot = snapshot
		if catchingUp {
			state.CatchupNext--
			if state.CatchupNext == state.NewestScheduled {
				state.NewestScheduled = state.CatchupTarget
				state.CatchupNext, state.CatchupTarget = -1, -1
			}
		} else {
			state.BackfillNext--
		}
		if err := store.Save(ctx, state, &record, activity); err != nil {
			return false, err
		}
		for _, resolution := range activity.Resolutions {
			if resolution.EvidenceErr != nil {
				configs.Logger.Warn("Slashing evidence missing; repair scheduled",
					zap.Int64("epoch", resolution.Epoch), zap.Strings("validatorIndices", resolution.ValidatorIndices), zap.Error(resolution.EvidenceErr))
			}
		}
	}
	return state.BackfillNext >= 0 || state.CatchupNext >= 0 || finalized > state.NewestScheduled, nil
}

func InitializeValidatorHistory() error {
	return slotAssignmentMutation(context.Background(), func(ctx context.Context) error {
		_, err := configs.ValidatorHistoryCollections.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{Keys: bson.D{{Key: "epoch", Value: 1}}, Options: options.Index().SetUnique(true).SetName("history_epoch_unique")},
			{Keys: bson.D{{Key: "source", Value: 1}, {Key: "epochInt", Value: -1}}, Options: options.Index().SetName("history_source_epoch")},
			{Keys: bson.D{{Key: "source", Value: 1}, {Key: "epochInt", Value: 1}, {Key: "slashingSnapshot.evidenceRetryAt", Value: 1}},
				Options: options.Index().SetName("history_evidence_repair").SetPartialFilterExpression(bson.M{"slashingSnapshot.evidenceMissing": true})},
		})
		if err != nil {
			return err
		}
		return initializeValidatorActivity(ctx)
	})
}

type mongoValidatorHistoryStore struct{}

func (mongoValidatorHistoryStore) Load(ctx context.Context) (validatorHistoryCursor, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var state validatorHistoryCursor
	err := configs.GetCollection(configs.DB, "sync_state").FindOne(ctx, bson.M{"_id": validatorHistoryCursorID}).Decode(&state)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return state, false, nil
	}
	return state, err == nil, err
}

func (mongoValidatorHistoryStore) Save(ctx context.Context, state validatorHistoryCursor, record *models.ValidatorHistoryRecord, activity validatorActivity) error {
	return slotAssignmentMutation(ctx, func(ctx context.Context) error {
		if err := storeValidatorActivity(ctx, activity); err != nil {
			return err
		}
		if record != nil {
			if err := services.StoreValidatorHistory(ctx, configs.ValidatorHistoryCollections, *record); err != nil {
				return err
			}
		}
		// Resolution markers follow every event/change upsert. A saved marker
		// also avoids repeating block requests after a cursor-write interruption.
		for _, resolution := range activity.Resolutions {
			if err := storeSlashingResolution(ctx, resolution); err != nil {
				return err
			}
		}
		// A crash after the upsert replays the same epoch. Persist progress last.
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := configs.GetCollection(configs.DB, "sync_state").UpdateOne(ctx,
			bson.M{"_id": validatorHistoryCursorID}, bson.M{"$set": state}, options.Update().SetUpsert(true))
		return err
	})
}

type validatorHistoryWorker struct {
	ejectionBalance      uint64
	warnedInvalidBalance bool
}

func (w *validatorHistoryWorker) pass(ctx context.Context, source interface {
	validatorHistorySource
	EjectionBalance(context.Context) (uint64, error)
}, store validatorHistoryStore, timing models.ChainTiming) (bool, error) {
	if w.ejectionBalance == 0 {
		value, err := source.EjectionBalance(ctx)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err != nil && !errors.Is(err, rpc.ErrInvalidEjectionBalance) {
			return false, err
		}
		if err != nil && !w.warnedInvalidBalance {
			configs.Logger.Warn("EJECTION_BALANCE unavailable; unslashed exits classified as exited until a valid spec is read", zap.Error(err))
			w.warnedInvalidBalance = true
		}
		w.ejectionBalance = value
	}
	return walkValidatorHistory(ctx, source, store, timing, w.ejectionBalance)
}

func StartValidatorHistorySync(stopCh <-chan struct{}) {
	worker := &validatorHistoryWorker{}
	chainClock.startWorker(stopCh, "Validator history", func(ctx context.Context) (bool, error) {
		return worker.pass(ctx, chainClock.beacon, mongoValidatorHistoryStore{}, chainClock.timing)
	})
}

// Registered writers drain before main releases the chain-indexer lease.
func (s *chainTimingState) startWorker(stopCh <-chan struct{}, name string, pass func(context.Context) (bool, error)) {
	startBackgroundWorker(func() {
		ctx, cancel := beaconWorkerContext(stopCh)
		defer cancel()
		if err := s.wait(ctx); err != nil {
			return
		}
		for ctx.Err() == nil {
			more, err := pass(ctx)
			if err != nil && ctx.Err() == nil {
				configs.Logger.Warn(name+" sync will retry", zap.Error(err))
			}
			if err == nil && more {
				continue
			}
			timer := time.NewTimer(30 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	})
}
