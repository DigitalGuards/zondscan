package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

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

func InsertValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert beacon response to storage format
	storage := models.ValidatorStorage{
		ID:         "validators", // Single document ID for easy updates
		Epoch:      currentEpoch,
		UpdatedAt:  fmt.Sprintf("%d", time.Now().Unix()),
		Validators: make([]models.ValidatorRecord, 0, len(beaconResponse.ValidatorList)),
	}

	// Convert each validator
	for _, v := range beaconResponse.ValidatorList {
		record := models.ValidatorRecord{
			Index:                      v.Index,
			PublicKeyHex:               models.Base64ToHex(v.Validator.PublicKey),
			WithdrawalCredentialsHex:   models.Base64ToHex(v.Validator.WithdrawalCredentials),
			EffectiveBalance:           v.Validator.EffectiveBalance,
			Slashed:                    v.Validator.Slashed,
			ActivationEligibilityEpoch: v.Validator.ActivationEligibilityEpoch,
			ActivationEpoch:            v.Validator.ActivationEpoch,
			ExitEpoch:                  v.Validator.ExitEpoch,
			WithdrawableEpoch:          v.Validator.WithdrawableEpoch,
			SlotNumber:                 v.Index, // Using index as slot number
			IsLeader:                   true,    // Set based on your leader selection logic
		}
		storage.Validators = append(storage.Validators, record)
	}

	// Upsert the document
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": "validators"}
	update := bson.M{"$set": storage}

	_, err := configs.ValidatorsCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update validator document", zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully updated validators",
		zap.Int("count", len(storage.Validators)),
		zap.String("epoch", currentEpoch))
	return nil
}

func GetValidators() (*models.ValidatorStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		configs.Logger.Error("Failed to get validator document", zap.Error(err))
		return nil, err
	}

	return &storage, nil
}

func GetValidatorByPublicKey(publicKeyHex string) (*models.ValidatorRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{
		"validators.publicKeyHex": publicKeyHex,
	}).Decode(&storage)

	if err != nil {
		return nil, err
	}

	// Find the matching validator
	for _, v := range storage.Validators {
		if v.PublicKeyHex == publicKeyHex {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("validator not found")
}

func GetBlockNumberFromHash(hash string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.hash": hash}
	options := options.FindOne().SetProjection(bson.M{"result.number": 1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to get block number from hash", zap.Error(err))
		return "0x0"
	}

	return block.Result.Number
}

// UpsertEpochInfo stores or updates the current epoch information
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

// GetEpochInfo retrieves the current epoch information
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

// InsertValidatorHistory inserts a validator history record for a specific epoch
func InsertValidatorHistory(record *models.ValidatorHistoryRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use epoch as unique identifier to prevent duplicate entries
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

// GetValidatorHistory retrieves historical validator data, optionally limited
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

// GetValidatorByIndex retrieves a validator by their index
func GetValidatorByIndex(index string) (*models.ValidatorRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{
		"validators.index": index,
	}).Decode(&storage)

	if err != nil {
		return nil, err
	}

	// Find the matching validator
	for _, v := range storage.Validators {
		if v.Index == index {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("validator not found")
}
