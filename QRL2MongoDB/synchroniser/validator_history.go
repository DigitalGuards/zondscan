package synchroniser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const validatorHistoryCursorID = "validator_history"
const validatorHistoryBatchSize = 16

// Two descending ranges keep recent finalized epochs ahead of the old backlog.
// -1 marks a finished range. NewestScheduled covers the original backlog and
// each completed catch-up range; BackfillNext retains unfinished older work.
type validatorHistoryCursor struct {
	BackfillNext    int64 `bson:"backfillNext"`
	NewestScheduled int64 `bson:"newestScheduled"`
	CatchupNext     int64 `bson:"catchupNext"`
	CatchupTarget   int64 `bson:"catchupTarget"`
}

type validatorHistorySource interface {
	FinalizedEpoch(context.Context) (int64, error)
	ValidatorsAtSlot(context.Context, int64) ([]models.StateValidator, error)
}

type validatorHistoryStore interface {
	Load(context.Context) (validatorHistoryCursor, bool, error)
	Save(context.Context, validatorHistoryCursor, *models.ValidatorHistoryRecord) error
}

func walkValidatorHistory(ctx context.Context, source validatorHistorySource, store validatorHistoryStore, timing models.ChainTiming) (bool, error) {
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
	if !exists {
		state = validatorHistoryCursor{BackfillNext: finalized, NewestScheduled: finalized, CatchupNext: -1, CatchupTarget: -1}
		if err := store.Save(ctx, state, nil); err != nil {
			return false, err
		}
	}
	if state.BackfillNext < -1 || state.BackfillNext > state.NewestScheduled || state.NewestScheduled > finalized || state.CatchupNext < -1 ||
		(state.CatchupNext >= 0 && (state.CatchupNext <= state.NewestScheduled || state.CatchupNext > state.CatchupTarget || state.CatchupTarget > finalized)) {
		return false, errors.New("invalid validator history cursor or regressed finality")
	}
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
		if catchingUp {
			state.CatchupNext--
			if state.CatchupNext == state.NewestScheduled {
				state.NewestScheduled = state.CatchupTarget
				state.CatchupNext, state.CatchupTarget = -1, -1
			}
		} else {
			state.BackfillNext--
		}
		if err := store.Save(ctx, state, &record); err != nil {
			return false, err
		}
	}
	return state.BackfillNext >= 0 || state.CatchupNext >= 0 || finalized > state.NewestScheduled, nil
}

func InitializeValidatorHistory() error {
	return slotAssignmentMutation(context.Background(), func(ctx context.Context) error {
		_, err := configs.ValidatorHistoryCollections.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{Keys: bson.D{{Key: "epoch", Value: 1}}, Options: options.Index().SetUnique(true).SetName("history_epoch_unique")},
			{Keys: bson.D{{Key: "source", Value: 1}, {Key: "epochInt", Value: -1}}, Options: options.Index().SetName("history_source_epoch")},
		})
		return err
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

func (mongoValidatorHistoryStore) Save(ctx context.Context, state validatorHistoryCursor, record *models.ValidatorHistoryRecord) error {
	return slotAssignmentMutation(ctx, func(ctx context.Context) error {
		if record != nil {
			if err := services.StoreValidatorHistory(ctx, configs.ValidatorHistoryCollections, *record); err != nil {
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

func StartValidatorHistorySync(stopCh <-chan struct{}) {
	chainClock.startWorker(stopCh, "Validator history", func(ctx context.Context) (bool, error) {
		return walkValidatorHistory(ctx, chainClock.beacon, mongoValidatorHistoryStore{}, chainClock.timing)
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
