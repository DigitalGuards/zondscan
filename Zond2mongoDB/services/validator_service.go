package services

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

// StoreValidators stores validator data from the beacon chain response
func StoreValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert each validator
	newValidators := make([]models.ValidatorRecord, 0, len(beaconResponse.ValidatorList))
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
		newValidators = append(newValidators, record)
	}

	// First try to get existing document
	var storage models.ValidatorStorage
	err := configs.GetValidatorCollection().FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil && err != mongo.ErrNoDocuments {
		configs.Logger.Error("Failed to get existing validator document", zap.Error(err))
		return err
	}

	if err == mongo.ErrNoDocuments {
		// Create new document if it doesn't exist
		storage = models.ValidatorStorage{
			ID:         "validators",
			Epoch:      currentEpoch,
			UpdatedAt:  fmt.Sprintf("%d", time.Now().Unix()),
			Validators: newValidators,
		}
	} else {
		// Append new validators to existing ones
		// First, create a map of existing validators by public key for deduplication
		existingValidators := make(map[string]bool)
		for _, v := range storage.Validators {
			existingValidators[v.PublicKeyHex] = true
		}

		// Only append validators that don't already exist
		for _, v := range newValidators {
			if !existingValidators[v.PublicKeyHex] {
				storage.Validators = append(storage.Validators, v)
			}
		}

		// Update epoch and timestamp
		storage.Epoch = currentEpoch
		storage.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	}

	// Upsert the document
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": "validators"}
	update := bson.M{"$set": storage}

	_, err = configs.GetValidatorCollection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update validator document", zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully updated validators",
		zap.Int("count", len(newValidators)),
		zap.String("epoch", currentEpoch))
	return nil
}

// GetValidators retrieves all validators from storage
func GetValidators() (*models.ValidatorStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.GetValidatorCollection().FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		configs.Logger.Error("Failed to get validator document", zap.Error(err))
		return nil, err
	}

	return &storage, nil
}

// GetValidatorByPublicKey retrieves a specific validator by their public key
func GetValidatorByPublicKey(publicKeyHex string) (*models.ValidatorRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.GetValidatorCollection().FindOne(ctx, bson.M{
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
