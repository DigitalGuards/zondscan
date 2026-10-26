package services

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StoreValidators updates the live set at the slot used to calculate currentEpoch.
// Historical summaries are written only by the finalized state walker.
func StoreValidators(ctx context.Context, validators []models.StateValidator, currentEpoch string) error {
	writes := make([]mongo.WriteModel, 0, len(validators))
	for _, v := range validators {
		if _, err := v.SummaryStatus(); err != nil {
			return err
		}
		doc := models.ValidatorDocument{
			ID:                         v.Index,
			PublicKeyHex:               strings.TrimPrefix(v.Validator.Pubkey, "0x"),
			WithdrawalCredentialsHex:   strings.TrimPrefix(v.Validator.WithdrawalRecipient, "0x"),
			EffectiveBalance:           v.Validator.EffectiveBalance,
			Slashed:                    v.Validator.Slashed,
			ActivationEligibilityEpoch: v.Validator.ActivationEligibilityEpoch,
			ActivationEpoch:            v.Validator.ActivationEpoch,
			ExitEpoch:                  v.Validator.ExitEpoch,
			WithdrawableEpoch:          v.Validator.WithdrawableEpoch,
			Epoch:                      currentEpoch, UpdatedAt: strconv.FormatInt(time.Now().Unix(), 10),
		}
		writes = append(writes, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": doc.ID}).SetUpdate(bson.M{"$set": doc}).SetUpsert(true))
	}
	if len(writes) == 0 {
		return nil
	}
	_, err := configs.ValidatorsCollections.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	return err
}

// StoreEpochInfo persists a v1 beacon head and its finality checkpoints.
func StoreEpochInfo(ctx context.Context, head *models.BeaconChainHeadResponse) error {
	info := models.EpochInfo{
		ID: "current", HeadEpoch: head.HeadEpoch, HeadSlot: head.HeadSlot,
		FinalizedEpoch: head.FinalizedEpoch, JustifiedEpoch: head.JustifiedEpoch,
		FinalizedSlot: head.FinalizedSlot, JustifiedSlot: head.JustifiedSlot,
		UpdatedAt: time.Now().Unix(),
	}
	_, err := configs.EpochInfoCollections.UpdateOne(ctx, bson.M{"_id": "current"}, bson.M{"$set": info}, options.Update().SetUpsert(true))
	return err
}

// BuildValidatorHistory summarizes the beacon state at the epoch's first slot.
func BuildValidatorHistory(timing models.ChainTiming, epoch int64, validators []models.StateValidator) (models.ValidatorHistoryRecord, error) {
	timestamp, err := timing.EpochTimestamp(epoch)
	if err != nil {
		return models.ValidatorHistoryRecord{}, err
	}
	record := models.ValidatorHistoryRecord{
		Epoch: strconv.FormatInt(epoch, 10), EpochInt: epoch, Timestamp: timestamp,
		Source: models.BeaconStateHistorySource, ValidatorsCount: len(validators),
	}
	total := new(big.Int)
	seen := make(map[uint64]bool, len(validators))
	for _, v := range validators {
		status, err := v.SummaryStatus()
		if err != nil {
			return models.ValidatorHistoryRecord{}, err
		}
		index, _ := strconv.ParseUint(v.Index, 10, 64)
		if seen[index] {
			return models.ValidatorHistoryRecord{}, fmt.Errorf("duplicate validator in history state")
		}
		seen[index] = true
		switch status {
		case "active":
			record.ActiveCount++
		case "pending":
			record.PendingCount++
		case "exited":
			record.ExitedCount++
		case "slashed":
			record.SlashedCount++
		}
		balance, ok := new(big.Int).SetString(v.Validator.EffectiveBalance, 10)
		if !ok || balance.Sign() < 0 {
			return models.ValidatorHistoryRecord{}, fmt.Errorf("invalid effective balance")
		}
		total.Add(total, balance)
	}
	record.TotalStaked = total.String()
	return record, nil
}

// StoreValidatorHistory replaces all summary fields on the existing epoch key,
// including legacy unmarked records. The caller owns the chain-indexer lease.
func StoreValidatorHistory(ctx context.Context, collection *mongo.Collection, record models.ValidatorHistoryRecord) error {
	if record.Source != models.BeaconStateHistorySource {
		return fmt.Errorf("validator history requires a beacon state")
	}
	_, err := collection.UpdateOne(ctx, bson.M{"epoch": record.Epoch}, bson.M{"$set": record}, options.Update().SetUpsert(true))
	return err
}
