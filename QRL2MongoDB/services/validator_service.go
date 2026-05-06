package services

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreValidators stores validator data from the beacon chain response.
// Each validator is written as its own MongoDB document keyed by its index.
func StoreValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
	if err := bulkUpsertValidators(beaconResponse, currentEpoch); err != nil {
		return err
	}

	currentEpochInt, _ := strconv.ParseInt(currentEpoch, 10, 64)
	if err := storeValidatorHistoryFromDB(currentEpoch, currentEpochInt); err != nil {
		configs.Logger.Warn("Failed to store validator history", zap.Error(err))
		// Do not fail the main operation for history errors.
	}

	configs.Logger.Info("Successfully updated validators",
		zap.Int("count", len(beaconResponse.ValidatorList)),
		zap.String("epoch", currentEpoch))
	return nil
}

// bulkUpsertValidators writes each validator as its own document using BulkWrite upserts.
func bulkUpsertValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
	if len(beaconResponse.ValidatorList) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	updatedAt := fmt.Sprintf("%d", time.Now().Unix())

	writeModels := make([]mongo.WriteModel, 0, len(beaconResponse.ValidatorList))
	for _, v := range beaconResponse.ValidatorList {
		doc := buildValidatorDocument(v, currentEpoch, updatedAt)
		filter := bson.M{"_id": doc.ID}
		update := bson.M{"$set": doc}
		writeModels = append(writeModels, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true))
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := configs.ValidatorsCollections.BulkWrite(ctx, writeModels, opts)
	if err != nil {
		configs.Logger.Error("Failed to bulk-write validator documents", zap.Error(err))
		return err
	}

	configs.Logger.Info("Bulk-upserted validators",
		zap.Int64("upserted", result.UpsertedCount),
		zap.Int64("modified", result.ModifiedCount),
		zap.String("epoch", currentEpoch))
	return nil
}

// buildValidatorDocument converts a BeaconValidator into a ValidatorDocument.
func buildValidatorDocument(v models.BeaconValidator, epoch, updatedAt string) models.ValidatorDocument {
	var idx int64
	fmt.Sscanf(v.Index, "%d", &idx)
	isLeader := idx%128 == 0 // Simplified leader selection

	return models.ValidatorDocument{
		ID:                         v.Index,
		PublicKeyHex:               models.Base64ToHex(v.Validator.PublicKey),
		WithdrawalCredentialsHex:   models.Base64ToHex(v.Validator.WithdrawalCredentials),
		EffectiveBalance:           v.Validator.EffectiveBalance,
		Slashed:                    v.Validator.Slashed,
		ActivationEligibilityEpoch: v.Validator.ActivationEligibilityEpoch,
		ActivationEpoch:            v.Validator.ActivationEpoch,
		ExitEpoch:                  v.Validator.ExitEpoch,
		WithdrawableEpoch:          v.Validator.WithdrawableEpoch,
		SlotNumber:                 v.Index,
		IsLeader:                   isLeader,
		Epoch:                      epoch,
		UpdatedAt:                  updatedAt,
	}
}

// storeValidatorHistoryFromDB computes validator statistics by scanning the
// per-document collection and persists them to validator_history.
func storeValidatorHistoryFromDB(epoch string, currentEpochInt int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	totalCount, err := configs.ValidatorsCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("count validators: %w", err)
	}

	// Project only the fields needed for status calculation and balance sum.
	cursor, err := configs.ValidatorsCollections.Find(ctx, bson.M{},
		options.Find().SetProjection(bson.M{
			"slashed":          1,
			"activationEpoch":  1,
			"exitEpoch":        1,
			"effectiveBalance": 1,
		}))
	if err != nil {
		return fmt.Errorf("find validators for history: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []models.ValidatorDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return fmt.Errorf("decode validators for history: %w", err)
	}

	var activeCount, pendingCount, exitedCount, slashedCount int
	totalStaked := big.NewInt(0)

	for _, d := range docs {
		status := models.GetValidatorStatus(d.ActivationEpoch, d.ExitEpoch, d.Slashed, currentEpochInt)
		switch status {
		case "active":
			activeCount++
		case "pending":
			pendingCount++
		case "exited":
			exitedCount++
		case "slashed":
			slashedCount++
		}
		if balance, ok := new(big.Int).SetString(d.EffectiveBalance, 10); ok {
			totalStaked.Add(totalStaked, balance)
		}
	}

	record := bson.M{
		"epoch":           epoch,
		"epochInt":        currentEpochInt,
		"timestamp":       time.Now().Unix(),
		"validatorsCount": int(totalCount),
		"activeCount":     activeCount,
		"pendingCount":    pendingCount,
		"exitedCount":     exitedCount,
		"slashedCount":    slashedCount,
		"totalStaked":     totalStaked.String(),
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"epoch": epoch}
	update := bson.M{"$set": record}

	_, err = configs.ValidatorHistoryCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("insert validator history: %w", err)
	}

	configs.Logger.Debug("Stored validator history",
		zap.String("epoch", epoch),
		zap.Int("validatorsCount", int(totalCount)))
	return nil
}

// StoreEpochInfo stores the current epoch information from beacon chain head.
func StoreEpochInfo(chainHead *models.BeaconChainHeadResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	epochInfo := &models.EpochInfo{
		ID:             "current",
		HeadEpoch:      chainHead.HeadEpoch,
		HeadSlot:       chainHead.HeadSlot,
		FinalizedEpoch: chainHead.FinalizedEpoch,
		JustifiedEpoch: chainHead.JustifiedEpoch,
		FinalizedSlot:  chainHead.FinalizedSlot,
		JustifiedSlot:  chainHead.JustifiedSlot,
		UpdatedAt:      time.Now().Unix(),
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": "current"}
	update := bson.M{"$set": epochInfo}

	_, err := configs.EpochInfoCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to upsert epoch info", zap.Error(err))
		return err
	}

	configs.Logger.Debug("Stored epoch info",
		zap.String("headEpoch", epochInfo.HeadEpoch),
		zap.String("headSlot", epochInfo.HeadSlot))
	return nil
}

// BackfillValidatorHistory fills in missing epoch records in validator_history.
// currentEpoch is the current epoch number derived from the latest block.
// Since all validators on this chain have been active since genesis with unchanged
// balances, we use the current validator state for all past epochs and compute
// timestamps from genesis time + epoch duration.
func BackfillValidatorHistory(currentEpoch int64) error {
	if currentEpoch <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const slotsPerEpoch = 128
	const secondsPerSlot = 60
	const epochDuration = int64(slotsPerEpoch * secondsPerSlot) // 7680s

	// Find which epochs already have records
	cursor, err := configs.ValidatorHistoryCollections.Find(ctx, bson.M{},
		options.Find().SetProjection(bson.M{"epoch": 1}))
	if err != nil {
		return fmt.Errorf("query existing history: %w", err)
	}
	defer cursor.Close(ctx)

	existingEpochs := make(map[string]bool)
	for cursor.Next(ctx) {
		var rec struct {
			Epoch string `bson:"epoch"`
		}
		if err := cursor.Decode(&rec); err == nil {
			existingEpochs[rec.Epoch] = true
		}
	}

	// If all epochs exist, nothing to do
	if int64(len(existingEpochs)) >= currentEpoch {
		configs.Logger.Debug("Validator history is complete, no backfill needed",
			zap.Int64("currentEpoch", currentEpoch),
			zap.Int("existingRecords", len(existingEpochs)))
		return nil
	}

	// Get current validator data
	totalCount, err := configs.ValidatorsCollections.CountDocuments(ctx, bson.M{})
	if err != nil || totalCount == 0 {
		return fmt.Errorf("no validators to backfill from: %w", err)
	}

	valCursor, err := configs.ValidatorsCollections.Find(ctx, bson.M{},
		options.Find().SetProjection(bson.M{
			"slashed": 1, "activationEpoch": 1, "exitEpoch": 1, "effectiveBalance": 1,
		}))
	if err != nil {
		return fmt.Errorf("find validators for backfill: %w", err)
	}
	defer valCursor.Close(ctx)

	var docs []models.ValidatorDocument
	if err := valCursor.All(ctx, &docs); err != nil {
		return fmt.Errorf("decode validators for backfill: %w", err)
	}

	// Get genesis time from block 0, falling back to block 1 minus one slot
	var blockDoc struct {
		Result struct {
			Timestamp string `bson:"timestamp"`
		} `bson:"result"`
	}
	genesisTime := int64(0)
	err = configs.BlocksCollections.FindOne(ctx, bson.M{"blockNumberInt": int64(0)}).Decode(&blockDoc)
	if err == nil && blockDoc.Result.Timestamp != "" {
		genesisTime, _ = strconv.ParseInt(blockDoc.Result.Timestamp, 0, 64)
	}
	if genesisTime == 0 {
		// Block 0 not in DB — derive genesis from block 1 (timestamp - 1 slot)
		err = configs.BlocksCollections.FindOne(ctx, bson.M{"blockNumberInt": int64(1)}).Decode(&blockDoc)
		if err == nil && blockDoc.Result.Timestamp != "" {
			t, _ := strconv.ParseInt(blockDoc.Result.Timestamp, 0, 64)
			genesisTime = t - secondsPerSlot
		}
	}
	if genesisTime == 0 {
		return fmt.Errorf("cannot backfill: neither block 0 nor block 1 found in database")
	}

	// Pre-parse validator balances to avoid repeated string→BigInt parsing in the loop
	type parsedValidator struct {
		ActivationEpoch string
		ExitEpoch       string
		Slashed         bool
		Balance         *big.Int
	}
	parsed := make([]parsedValidator, 0, len(docs))
	for _, d := range docs {
		bal := big.NewInt(0)
		if b, ok := new(big.Int).SetString(d.EffectiveBalance, 10); ok {
			bal = b
		}
		parsed = append(parsed, parsedValidator{
			ActivationEpoch: d.ActivationEpoch,
			ExitEpoch:       d.ExitEpoch,
			Slashed:         d.Slashed,
			Balance:         bal,
		})
	}

	// Build batch of missing epoch records
	writeModels := make([]mongo.WriteModel, 0)
	for epoch := int64(0); epoch < currentEpoch; epoch++ {
		epochStr := strconv.FormatInt(epoch, 10)
		if existingEpochs[epochStr] {
			continue
		}

		var activeCount, pendingCount, exitedCount, slashedCount int
		totalStaked := big.NewInt(0)
		for _, v := range parsed {
			status := models.GetValidatorStatus(v.ActivationEpoch, v.ExitEpoch, v.Slashed, epoch)
			switch status {
			case "active":
				activeCount++
			case "pending":
				pendingCount++
			case "exited":
				exitedCount++
			case "slashed":
				slashedCount++
			}
			totalStaked.Add(totalStaked, v.Balance)
		}

		doc := bson.M{
			"epoch":           epochStr,
			"epochInt":        epoch,
			"timestamp":       genesisTime + epoch*epochDuration,
			"validatorsCount": int(totalCount),
			"activeCount":     activeCount,
			"pendingCount":    pendingCount,
			"exitedCount":     exitedCount,
			"slashedCount":    slashedCount,
			"totalStaked":     totalStaked.String(),
		}

		filter := bson.M{"epoch": epochStr}
		update := bson.M{"$set": doc}
		writeModels = append(writeModels, mongo.NewUpdateOneModel().
			SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}

	if len(writeModels) == 0 {
		return nil
	}

	result, err := configs.ValidatorHistoryCollections.BulkWrite(ctx, writeModels, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("bulk write backfill: %w", err)
	}

	configs.Logger.Info("Backfilled validator history",
		zap.Int64("upserted", result.UpsertedCount),
		zap.Int64("modified", result.ModifiedCount),
		zap.Int64("currentEpoch", currentEpoch))
	return nil
}

