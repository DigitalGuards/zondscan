package synchroniser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const slotAssignmentBatchSize = 16

type slotAssignmentSource interface {
	FinalizedEpoch(context.Context) (int64, error)
	Assignments(context.Context, int64) ([]models.SlotAssignment, error)
}

type slotAssignmentStore interface {
	Load(context.Context) (models.SlotAssignmentCursor, bool, error)
	Save(context.Context, models.SlotAssignmentCursor, []models.SlotAssignment) error
}

// walkSlotAssignments makes bounded progress and reloads its durable cursor on
// every pass. A failed epoch is replayed after restart, including partial writes.
func walkSlotAssignments(ctx context.Context, source slotAssignmentSource, store slotAssignmentStore, timing models.ChainTiming) (bool, error) {
	if err := timing.Validate(); err != nil {
		return false, err
	}
	state, exists, err := store.Load(ctx)
	if err != nil {
		return false, err
	}
	if !exists {
		state.GenesisTime = timing.GenesisTime
		if err := store.Save(ctx, state, nil); err != nil {
			return false, err
		}
	}
	if state.GenesisTime <= 0 || state.GenesisTime != timing.GenesisTime || state.NextEpoch < 0 {
		return false, errors.New("invalid slot assignment cursor")
	}
	finalized, err := source.FinalizedEpoch(ctx)
	if err != nil {
		return false, err
	}
	for count := 0; count < slotAssignmentBatchSize && state.NextEpoch <= finalized; count++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		epoch := state.NextEpoch
		rows, err := source.Assignments(ctx, epoch)
		if errors.Is(err, rpc.ErrEpochUnavailable) {
			state.UnavailableEpochs++
			rows = nil
		} else if err != nil {
			return false, fmt.Errorf("epoch %d: %w", epoch, err)
		} else {
			rows, err = rpc.ValidateSlotAssignments(timing, epoch, rows)
			if err != nil {
				return false, fmt.Errorf("epoch %d: %w", epoch, err)
			}
			if state.FirstAvailableEpoch == nil {
				state.FirstAvailableEpoch = &epoch
			}
		}
		state.NextEpoch++
		if err := store.Save(ctx, state, rows); err != nil {
			return false, err
		}
	}
	return state.NextEpoch <= finalized, nil
}

// slotAssignmentMutation uses the existing chain writer lock and lease. Its
// deadline ends before lease expiry, including when a renewal stops mid-write.
func slotAssignmentMutation(ctx context.Context, fn func(context.Context) error) error {
	chainMutationMu.Lock()
	defer chainMutationMu.Unlock()
	expiry := chainMutationLeaseExpiry.Load()
	deadline := time.Unix(0, expiry).Add(-time.Second)
	if expiry == 0 || !time.Now().Before(deadline) {
		return errors.New("beacon writer requires an active chain-indexer lease")
	}
	if cap := time.Now().Add(10 * time.Second); cap.Before(deadline) {
		deadline = cap
	}
	writeCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if err := writeCtx.Err(); err != nil {
		return err
	}
	return fn(writeCtx)
}

// InitializeSlotAssignments is a fail-closed leased startup prerequisite.
func InitializeSlotAssignments() error {
	if err := slotAssignmentMutation(context.Background(), func(ctx context.Context) error {
		_, err := configs.SlotAssignmentsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{Keys: bson.D{{Key: "slot", Value: 1}}, Options: options.Index().SetUnique(true).SetName("slot_unique")},
			{Keys: bson.D{{Key: "epoch", Value: 1}}, Options: options.Index().SetName("epoch_idx")},
		})
		return err
	}); err != nil {
		return err
	}
	return slotAssignmentMutation(context.Background(), func(ctx context.Context) error {
		_, err := configs.BlocksCollections.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "result.timestamp", Value: 1}, {Key: "ingestionState", Value: 1}},
			Options: options.Index().SetName("slot_timestamp_ingestion_idx"),
		})
		return err
	})
}

type mongoSlotAssignmentStore struct{}

func (mongoSlotAssignmentStore) Load(ctx context.Context) (models.SlotAssignmentCursor, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var state models.SlotAssignmentCursor
	err := configs.GetCollection(configs.DB, "sync_state").FindOne(ctx, bson.M{"_id": models.SlotAssignmentCursorID}).Decode(&state)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return state, false, nil
	}
	return state, err == nil, err
}

func (mongoSlotAssignmentStore) Save(ctx context.Context, state models.SlotAssignmentCursor, rows []models.SlotAssignment) error {
	return slotAssignmentMutation(ctx, func(ctx context.Context) error {
		if len(rows) > 0 {
			writes := make([]mongo.WriteModel, 0, len(rows))
			for _, row := range rows {
				writes = append(writes, mongo.NewUpdateOneModel().SetFilter(bson.M{"slot": row.Slot}).SetUpdate(bson.M{"$set": row}).SetUpsert(true))
			}
			if _, err := configs.SlotAssignmentsCollection.BulkWrite(ctx, writes); err != nil {
				return err
			}
		}
		// Cursor persistence follows every successful assignment write. Upserts
		// make a crash between these operations safe to replay on standalone Mongo.
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := configs.GetCollection(configs.DB, "sync_state").UpdateOne(ctx,
			bson.M{"_id": models.SlotAssignmentCursorID}, bson.M{"$set": state}, options.Update().SetUpsert(true))
		return err
	})
}

// StartSlotAssignmentSync stays registered through cancellation and draining so
// main can release the chain-indexer lease after this writer has stopped.
func StartSlotAssignmentSync(stopCh <-chan struct{}) {
	chainClock.startWorker(stopCh, "Slot assignments", func(ctx context.Context) (bool, error) {
		return walkSlotAssignments(ctx, chainClock.beacon, mongoSlotAssignmentStore{}, chainClock.timing)
	})
}
