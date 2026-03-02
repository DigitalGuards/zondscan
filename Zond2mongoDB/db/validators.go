package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// UpdateValidators updates the previousHash field on a block document.
func UpdateValidators(blockNumber string, previousHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.number": blockNumber}
	update := bson.M{"$set": bson.M{"previousHash": previousHash}}

	_, err := configs.BlocksCollections.UpdateOne(ctx, filter, update)
	if err != nil {
		configs.Logger.Info("Failed to update validator document", zap.Error(err))
	}
}

// InsertValidators stores each validator as its own document using BulkWrite upserts.
// The document _id is the validator index string. This replaces the legacy single
// mega-document approach and avoids MongoDB's 16 MB document size limit.
func InsertValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
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

	configs.Logger.Info("Successfully upserted validators",
		zap.Int64("upserted", result.UpsertedCount),
		zap.Int64("modified", result.ModifiedCount),
		zap.String("epoch", currentEpoch))
	return nil
}

// buildValidatorDocument converts a BeaconValidator into a ValidatorDocument.
func buildValidatorDocument(v models.BeaconValidator, epoch, updatedAt string) models.ValidatorDocument {
	slotNum := v.Index
	isLeader := false
	// Simplified leader selection: every 128th index slot is a leader.
	var idx int64
	fmt.Sscanf(v.Index, "%d", &idx)
	isLeader = idx%128 == 0

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
		SlotNumber:                 slotNum,
		IsLeader:                   isLeader,
		Epoch:                      epoch,
		UpdatedAt:                  updatedAt,
	}
}

// GetValidators retrieves all validator documents from the collection and assembles them
// into the legacy ValidatorStorage shape so the rest of the syncer pipeline is unaffected.
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

	// Convert []ValidatorDocument → []ValidatorRecord for callers that still use ValidatorStorage.
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
		ID:         "validators",
		Epoch:      epoch,
		UpdatedAt:  updatedAt,
		Validators: records,
	}, nil
}

// GetValidatorByPublicKey retrieves a single validator by its hex public key.
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

// GetValidatorByIndex retrieves a validator by its index string.
func GetValidatorByIndex(index string) (*models.ValidatorRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var doc models.ValidatorDocument
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": index}).Decode(&doc)
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

// GetBlockNumberFromHash returns the block number for a given block hash.
func GetBlockNumberFromHash(hash string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.hash": hash}
	findOpts := options.FindOne().SetProjection(bson.M{"result.number": 1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter, findOpts).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to get block number from hash", zap.Error(err))
		return "0x0"
	}

	return block.Result.Number
}

// UpsertEpochInfo stores or updates the current epoch information.
func UpsertEpochInfo(epochInfo *models.EpochInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	epochInfo.ID = "current"
	epochInfo.UpdatedAt = time.Now().Unix()

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": "current"}
	update := bson.M{"$set": epochInfo}

	_, err := configs.EpochInfoCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to upsert epoch info", zap.Error(err))
		return err
	}

	configs.Logger.Debug("Upserted epoch info",
		zap.String("headEpoch", epochInfo.HeadEpoch),
		zap.String("headSlot", epochInfo.HeadSlot))
	return nil
}

// GetEpochInfo retrieves the current epoch information.
func GetEpochInfo() (*models.EpochInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var epochInfo models.EpochInfo
	err := configs.EpochInfoCollections.FindOne(ctx, bson.M{"_id": "current"}).Decode(&epochInfo)
	if err != nil {
		configs.Logger.Error("Failed to get epoch info", zap.Error(err))
		return nil, err
	}

	return &epochInfo, nil
}

// InsertValidatorHistory inserts a validator history record for a specific epoch.
func InsertValidatorHistory(record *models.ValidatorHistoryRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"epoch": record.Epoch}
	update := bson.M{"$set": record}

	_, err := configs.ValidatorHistoryCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to insert validator history", zap.Error(err))
		return err
	}

	configs.Logger.Debug("Inserted validator history",
		zap.String("epoch", record.Epoch),
		zap.Int("validatorsCount", record.ValidatorsCount))
	return nil
}

// GetValidatorHistory retrieves historical validator data, optionally limited.
func GetValidatorHistory(limit int) ([]models.ValidatorHistoryRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOpts := options.Find().SetSort(bson.D{{Key: "epoch", Value: -1}})
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}

	cursor, err := configs.ValidatorHistoryCollections.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		configs.Logger.Error("Failed to get validator history", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	var history []models.ValidatorHistoryRecord
	if err := cursor.All(ctx, &history); err != nil {
		return nil, err
	}

	return history, nil
}
