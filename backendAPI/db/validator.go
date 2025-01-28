package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
		// Calculate if validator is active
		activationEpoch := HexToInt(v.ActivationEpoch)
		exitEpoch := HexToInt(v.ExitEpoch)
		isActive := activationEpoch <= currentEpoch && currentEpoch < exitEpoch

		// Calculate age in epochs
		age := int64(0)
		if activationEpoch <= currentEpoch {
			age = currentEpoch - activationEpoch
		}

		// Add validator to response
		validator := models.Validator{
			Address:      v.PublicKeyHex,
			Uptime:       100.0, // TODO: Calculate actual uptime
			Age:          age,
			StakedAmount: v.EffectiveBalance,
			IsActive:     isActive,
		}
		validators = append(validators, validator)

		// Add to total staked
		totalStaked += HexToInt(v.EffectiveBalance)
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
