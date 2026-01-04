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

// StoreValidators stores validator data from the beacon chain response
func StoreValidators(beaconResponse models.BeaconValidatorResponse, currentEpoch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse current epoch for status calculation
	currentEpochInt, _ := strconv.ParseInt(currentEpoch, 10, 64)

	// Convert each validator
	newValidators := make([]models.ValidatorRecord, 0, len(beaconResponse.ValidatorList))
	for _, v := range beaconResponse.ValidatorList {
		// Determine if this validator is the leader for their slot (simplified: based on index mod)
		slotNum, _ := strconv.ParseInt(v.Index, 10, 64)
		isLeader := slotNum%128 == 0 // Simplified leader selection

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
			SlotNumber:                 v.Index,
			IsLeader:                   isLeader,
		}
		newValidators = append(newValidators, record)
	}

	// Store validator history for this epoch
	if err := StoreValidatorHistory(newValidators, currentEpoch, currentEpochInt); err != nil {
		configs.Logger.Warn("Failed to store validator history", zap.Error(err))
		// Don't fail the main operation
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

// StoreEpochInfo stores the current epoch information from beacon chain head
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
func StoreValidatorHistory(validators []models.ValidatorRecord, epoch string, currentEpochInt int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var activeCount, pendingCount, exitedCount, slashedCount int
	totalStaked := big.NewInt(0)

	for _, v := range validators {
		// Calculate status
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

		// Sum effective balance
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

	// Use epoch as unique identifier to prevent duplicate entries
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
