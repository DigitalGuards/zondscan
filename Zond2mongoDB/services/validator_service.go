package services

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
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

	record := &models.ValidatorHistoryRecord{
		Epoch:           epoch,
		Timestamp:       time.Now().Unix(),
		ValidatorsCount: int(totalCount),
		ActiveCount:     activeCount,
		PendingCount:    pendingCount,
		ExitedCount:     exitedCount,
		SlashedCount:    slashedCount,
		TotalStaked:     totalStaked.String(),
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"epoch": record.Epoch}
	update := bson.M{"$set": record}

	_, err = configs.ValidatorHistoryCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("insert validator history: %w", err)
	}

	configs.Logger.Debug("Stored validator history",
		zap.String("epoch", record.Epoch),
		zap.Int("validatorsCount", record.ValidatorsCount))
	return nil
}

// GetValidators retrieves all validators from the per-document collection
// and returns them assembled in the legacy ValidatorStorage shape.
func GetValidators() (*models.ValidatorStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := configs.ValidatorsCollections.Find(ctx, bson.M{})
	if err != nil {
		configs.Logger.Error("Failed to find validator documents", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []models.ValidatorDocument
	if err := cursor.All(ctx, &docs); err != nil {
		configs.Logger.Error("Failed to decode validator documents", zap.Error(err))
		return nil, err
	}

	records := make([]models.ValidatorRecord, 0, len(docs))
	epoch := ""
	updatedAt := ""
	for _, d := range docs {
		records = append(records, validatorDocToRecord(d))
		if epoch == "" {
			epoch = d.Epoch
			updatedAt = d.UpdatedAt
		}
	}

	return &models.ValidatorStorage{
		ID:        "validators",
		Epoch:     epoch,
		UpdatedAt: updatedAt,
		Validators: records,
	}, nil
}

// GetValidatorByPublicKey retrieves a specific validator by their public key hex.
func GetValidatorByPublicKey(publicKeyHex string) (*models.ValidatorRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var doc models.ValidatorDocument
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"publicKeyHex": publicKeyHex}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("validator not found")
		}
		return nil, err
	}

	record := validatorDocToRecord(doc)
	return &record, nil
}

// validatorDocToRecord maps a ValidatorDocument to the legacy ValidatorRecord type.
func validatorDocToRecord(d models.ValidatorDocument) models.ValidatorRecord {
	return models.ValidatorRecord{
		Index:                      d.ID,
		PublicKeyHex:               d.PublicKeyHex,
		WithdrawalCredentialsHex:   d.WithdrawalCredentialsHex,
		EffectiveBalance:           d.EffectiveBalance,
		Slashed:                    d.Slashed,
		ActivationEligibilityEpoch: d.ActivationEligibilityEpoch,
		ActivationEpoch:            d.ActivationEpoch,
		ExitEpoch:                  d.ExitEpoch,
		WithdrawableEpoch:          d.WithdrawableEpoch,
		SlotNumber:                 d.SlotNumber,
		IsLeader:                   d.IsLeader,
	}
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

// StoreValidatorHistory computes and stores validator statistics for the current epoch
// from a supplied []ValidatorRecord slice (kept for callers that already have the data).
func StoreValidatorHistory(validators []models.ValidatorRecord, epoch string, currentEpochInt int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var activeCount, pendingCount, exitedCount, slashedCount int
	totalStaked := big.NewInt(0)

	for _, v := range validators {
		status := models.GetValidatorStatus(v.ActivationEpoch, v.ExitEpoch, v.Slashed, currentEpochInt)
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
		if balance, ok := new(big.Int).SetString(v.EffectiveBalance, 10); ok {
			totalStaked.Add(totalStaked, balance)
		}
	}

	record := &models.ValidatorHistoryRecord{
		Epoch:           epoch,
		Timestamp:       time.Now().Unix(),
		ValidatorsCount: len(validators),
		ActiveCount:     activeCount,
		PendingCount:    pendingCount,
		ExitedCount:     exitedCount,
		SlashedCount:    slashedCount,
		TotalStaked:     totalStaked.String(),
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"epoch": record.Epoch}
	update := bson.M{"$set": record}

	_, err := configs.ValidatorHistoryCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to insert validator history", zap.Error(err))
		return err
	}

	configs.Logger.Debug("Stored validator history",
		zap.String("epoch", record.Epoch),
		zap.Int("validatorsCount", record.ValidatorsCount))
	return nil
}
