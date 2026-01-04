package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	SlotsPerEpoch  = 128
	SecondsPerSlot = 60
)

func ReturnValidators(pageToken string) (*models.ValidatorResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the validator document
	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Return empty response if no validators found
			return &models.ValidatorResponse{
				Validators:  make([]models.Validator, 0),
				TotalStaked: "0",
			}, nil
		}
		return nil, fmt.Errorf("failed to get validator document: %v", err)
	}

	// Get current epoch from latest block
	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}

	// Convert hex block number to int for epoch calculation
	currentEpoch := HexToInt(latestBlock) / 128 // Each epoch is 128 blocks

	// Process validators
	validators := make([]models.Validator, 0)
	totalStaked := int64(0)

	for _, v := range storage.Validators {
		// Calculate status
		status := getValidatorStatus(v.ActivationEpoch, v.ExitEpoch, v.Slashed, currentEpoch)
		isActive := status == "active"

		// Calculate age in epochs
		activationEpoch := parseEpoch(v.ActivationEpoch)
		age := int64(0)
		if activationEpoch <= currentEpoch {
			age = currentEpoch - activationEpoch
		}

		// Add validator to response
		validator := models.Validator{
			Index:        v.Index,
			Address:      v.PublicKeyHex,
			Status:       status,
			Age:          age,
			StakedAmount: v.EffectiveBalance,
			IsActive:     isActive,
		}
		validators = append(validators, validator)

		// Add to total staked (parse as decimal since syncer stores decimal)
		if balance, err := strconv.ParseInt(v.EffectiveBalance, 10, 64); err == nil {
			totalStaked += balance
		}
	}

	return &models.ValidatorResponse{
		Validators:  validators,
		TotalStaked: fmt.Sprintf("%d", totalStaked),
	}, nil
}

// CountValidators returns the total number of validators
func CountValidators() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get validator document: %v", err)
	}

	return int64(len(storage.Validators)), nil
}

// Helper function to convert hex string to int64
func HexToInt(hex string) int64 {
	// Remove "0x" prefix if present
	if len(hex) > 2 && hex[0:2] == "0x" {
		hex = hex[2:]
	}

	// Parse hex string
	var result int64
	fmt.Sscanf(hex, "%x", &result)
	return result
}

// parseEpoch parses epoch string (handles both hex and decimal formats)
// FAR_FUTURE_EPOCH represents a validator that hasn't exited
const FAR_FUTURE_EPOCH = "18446744073709551615"

func parseEpoch(epochStr string) int64 {
	// FAR_FUTURE_EPOCH is special - return max int64 to indicate "never"
	if epochStr == FAR_FUTURE_EPOCH {
		return int64(^uint64(0) >> 1) // Max int64: 9223372036854775807
	}
	// Try decimal first
	if epoch, err := strconv.ParseInt(epochStr, 10, 64); err == nil {
		return epoch
	}
	// Try hex
	return HexToInt(epochStr)
}

// getValidatorStatus computes the validator status based on current epoch
func getValidatorStatus(activationEpoch, exitEpoch string, slashed bool, currentEpoch int64) string {
	activation := parseEpoch(activationEpoch)
	exit := parseEpoch(exitEpoch)

	if slashed {
		return "slashed"
	}
	if activation > currentEpoch {
		return "pending"
	}
	if exit <= currentEpoch {
		return "exited"
	}
	return "active"
}

// GetEpochInfo retrieves the current epoch information
func GetEpochInfo() (*models.EpochInfoResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var epochInfo models.EpochInfo
	err := configs.EpochInfoCollection.FindOne(ctx, bson.M{"_id": "current"}).Decode(&epochInfo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("epoch info not found")
		}
		return nil, fmt.Errorf("failed to get epoch info: %v", err)
	}

	// Calculate slot within epoch and time to next epoch
	headSlot := parseEpoch(epochInfo.HeadSlot)
	slotInEpoch := headSlot % SlotsPerEpoch
	slotsRemaining := SlotsPerEpoch - slotInEpoch
	timeToNextEpoch := slotsRemaining * SecondsPerSlot

	return &models.EpochInfoResponse{
		HeadEpoch:       epochInfo.HeadEpoch,
		HeadSlot:        epochInfo.HeadSlot,
		FinalizedEpoch:  epochInfo.FinalizedEpoch,
		JustifiedEpoch:  epochInfo.JustifiedEpoch,
		SlotsPerEpoch:   SlotsPerEpoch,
		SecondsPerSlot:  SecondsPerSlot,
		SlotInEpoch:     slotInEpoch,
		TimeToNextEpoch: timeToNextEpoch,
		UpdatedAt:       epochInfo.UpdatedAt,
	}, nil
}

// GetValidatorHistory retrieves historical validator data
func GetValidatorHistory(limit int) (*models.ValidatorHistoryResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOpts := options.Find().SetSort(bson.D{{Key: "epoch", Value: -1}})
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}

	cursor, err := configs.ValidatorHistoryCollection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator history: %v", err)
	}
	defer cursor.Close(ctx)

	var history []models.ValidatorHistoryRecord
	if err := cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("failed to decode validator history: %v", err)
	}

	return &models.ValidatorHistoryResponse{
		History: history,
	}, nil
}

// GetValidatorByID retrieves a validator by index or public key
func GetValidatorByID(id string) (*models.ValidatorDetailResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("validators not found")
		}
		return nil, fmt.Errorf("failed to get validators: %v", err)
	}

	// Get current epoch
	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := HexToInt(latestBlock) / 128

	// Find validator by index or public key
	for _, v := range storage.Validators {
		if v.Index == id || v.PublicKeyHex == id {
			status := getValidatorStatus(v.ActivationEpoch, v.ExitEpoch, v.Slashed, currentEpoch)
			activationEpoch := parseEpoch(v.ActivationEpoch)
			age := int64(0)
			if activationEpoch <= currentEpoch {
				age = currentEpoch - activationEpoch
			}

			return &models.ValidatorDetailResponse{
				Index:                      v.Index,
				PublicKeyHex:               v.PublicKeyHex,
				WithdrawalCredentialsHex:   v.WithdrawalCredentialsHex,
				EffectiveBalance:           v.EffectiveBalance,
				Slashed:                    v.Slashed,
				ActivationEligibilityEpoch: v.ActivationEligibilityEpoch,
				ActivationEpoch:            v.ActivationEpoch,
				ExitEpoch:                  v.ExitEpoch,
				WithdrawableEpoch:          v.WithdrawableEpoch,
				Status:                     status,
				Age:                        age,
				CurrentEpoch:               fmt.Sprintf("%d", currentEpoch),
			}, nil
		}
	}

	return nil, fmt.Errorf("validator not found")
}

// GetValidatorStats returns aggregated validator statistics
func GetValidatorStats() (*models.ValidatorStatsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storage models.ValidatorStorage
	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "validators"}).Decode(&storage)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &models.ValidatorStatsResponse{}, nil
		}
		return nil, fmt.Errorf("failed to get validators: %v", err)
	}

	// Get current epoch
	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := HexToInt(latestBlock) / 128

	var activeCount, pendingCount, exitedCount, slashedCount int
	totalStaked := int64(0)

	for _, v := range storage.Validators {
		status := getValidatorStatus(v.ActivationEpoch, v.ExitEpoch, v.Slashed, currentEpoch)
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

		if balance, err := strconv.ParseInt(v.EffectiveBalance, 10, 64); err == nil {
			totalStaked += balance
		}
	}

	return &models.ValidatorStatsResponse{
		TotalValidators: len(storage.Validators),
		ActiveCount:     activeCount,
		PendingCount:    pendingCount,
		ExitedCount:     exitedCount,
		SlashedCount:    slashedCount,
		TotalStaked:     fmt.Sprintf("%d", totalStaked),
		CurrentEpoch:    fmt.Sprintf("%d", currentEpoch),
	}, nil
}
