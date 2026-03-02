package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"math"
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

// ReturnValidators returns all validators with computed status and totals.
// It queries the per-document validators collection directly instead of loading
// a single mega-document and iterating in Go.
func ReturnValidators(pageToken string) (*models.ValidatorResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current epoch from latest block once; reuse for all validators.
	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := HexToInt(latestBlock) / 128

	findOpts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})

	cursor, err := configs.ValidatorsCollections.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &models.ValidatorResponse{
				Validators:  make([]models.Validator, 0),
				TotalStaked: "0",
			}, nil
		}
		return nil, fmt.Errorf("failed to query validators: %v", err)
	}
	defer cursor.Close(ctx)

	var docs []models.ValidatorDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode validators: %v", err)
	}

	validators := make([]models.Validator, 0, len(docs))
	totalStaked := int64(0)

	for _, d := range docs {
		status := getValidatorStatus(d.ActivationEpoch, d.ExitEpoch, d.Slashed, currentEpoch)
		isActive := status == "active"

		activationEpoch := parseEpoch(d.ActivationEpoch)
		age := int64(0)
		if activationEpoch <= currentEpoch {
			age = currentEpoch - activationEpoch
		}

		validators = append(validators, models.Validator{
			Index:        d.ID,
			Address:      d.PublicKeyHex,
			Status:       status,
			Age:          age,
			StakedAmount: d.EffectiveBalance,
			IsActive:     isActive,
		})

		if balance, err := strconv.ParseInt(d.EffectiveBalance, 10, 64); err == nil {
			totalStaked += balance
		}
	}

	return &models.ValidatorResponse{
		Validators:  validators,
		TotalStaked: fmt.Sprintf("%d", totalStaked),
	}, nil
}

// CountValidators returns the total number of validator documents in the collection.
func CountValidators() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.ValidatorsCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count validators: %v", err)
	}
	return count, nil
}

// Helper function to convert hex string to int64.
func HexToInt(hex string) int64 {
	if len(hex) > 2 && hex[0:2] == "0x" {
		hex = hex[2:]
	}
	var result int64
	fmt.Sscanf(hex, "%x", &result)
	return result
}

// FAR_FUTURE_EPOCH represents a validator that hasn't exited.
const FAR_FUTURE_EPOCH = "18446744073709551615"

// parseEpoch parses an epoch string (handles decimal format and FAR_FUTURE_EPOCH).
func parseEpoch(epochStr string) int64 {
	if epochStr == FAR_FUTURE_EPOCH {
		return math.MaxInt64
	}
	if epoch, err := strconv.ParseInt(epochStr, 10, 64); err == nil {
		return epoch
	}
	return HexToInt(epochStr)
}

// getValidatorStatus computes the validator status based on current epoch.
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

// GetEpochInfo retrieves the current epoch information.
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

// GetValidatorHistory retrieves historical validator data.
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

// GetValidatorByID retrieves a validator by index (decimal string) or public key hex.
// Uses a direct document lookup instead of loading all validators into memory.
func GetValidatorByID(id string) (*models.ValidatorDetailResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := HexToInt(latestBlock) / 128

	// Try lookup by _id (index) first, then fall back to publicKeyHex.
	filter := bson.M{"$or": []bson.M{
		{"_id": id},
		{"publicKeyHex": id},
	}}

	var doc models.ValidatorDocument
	err = configs.ValidatorsCollections.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("validator not found")
		}
		return nil, fmt.Errorf("failed to get validator: %v", err)
	}

	status := getValidatorStatus(doc.ActivationEpoch, doc.ExitEpoch, doc.Slashed, currentEpoch)
	activationEpoch := parseEpoch(doc.ActivationEpoch)
	age := int64(0)
	if activationEpoch <= currentEpoch {
		age = currentEpoch - activationEpoch
	}

	return &models.ValidatorDetailResponse{
		Index:                      doc.ID,
		PublicKeyHex:               doc.PublicKeyHex,
		WithdrawalCredentialsHex:   doc.WithdrawalCredentialsHex,
		EffectiveBalance:           doc.EffectiveBalance,
		Slashed:                    doc.Slashed,
		ActivationEligibilityEpoch: doc.ActivationEligibilityEpoch,
		ActivationEpoch:            doc.ActivationEpoch,
		ExitEpoch:                  doc.ExitEpoch,
		WithdrawableEpoch:          doc.WithdrawableEpoch,
		Status:                     status,
		Age:                        age,
		CurrentEpoch:               fmt.Sprintf("%d", currentEpoch),
	}, nil
}

// GetValidatorStats returns aggregated validator statistics using a MongoDB aggregation
// pipeline instead of loading all validators into Go memory.
func GetValidatorStats() (*models.ValidatorStatsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latestBlock, err := GetLatestBlockFromSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %v", err)
	}
	currentEpoch := HexToInt(latestBlock) / 128

	// Check whether the collection has any documents at all.
	totalCount, err := configs.ValidatorsCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to count validators: %v", err)
	}
	if totalCount == 0 {
		return &models.ValidatorStatsResponse{
			CurrentEpoch: fmt.Sprintf("%d", currentEpoch),
		}, nil
	}

	// Use aggregation to compute per-status counts and total staked in one pass.
	// Status computation requires knowing currentEpoch, which MongoDB doesn't know,
	// so we project the fields needed and compute buckets in a $group stage using
	// $cond expressions that mirror getValidatorStatus logic.
	currentEpochStr := fmt.Sprintf("%d", currentEpoch)

	pipeline := mongo.Pipeline{
		// Add a computed "status" field using the same rules as getValidatorStatus.
		bson.D{{Key: "$addFields", Value: bson.M{
			"_computedStatus": bson.M{
				"$switch": bson.M{
					"branches": []bson.M{
						{
							// slashed
							"case":  bson.M{"$eq": []interface{}{"$slashed", true}},
							"then":  "slashed",
						},
						{
							// pending: activationEpoch > currentEpoch
							"case": bson.M{"$gt": []interface{}{"$activationEpoch", currentEpochStr}},
							"then": "pending",
						},
						{
							// exited: exitEpoch <= currentEpoch AND exitEpoch != FAR_FUTURE_EPOCH
							"case": bson.M{"$and": []bson.M{
								{"$lte": []interface{}{"$exitEpoch", currentEpochStr}},
								{"$ne": []interface{}{"$exitEpoch", FAR_FUTURE_EPOCH}},
							}},
							"then": "exited",
						},
					},
					"default": "active",
				},
			},
		}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":          "$_computedStatus",
			"count":        bson.M{"$sum": 1},
			// We sum effective balance as strings; MongoDB can't do numeric sum on
			// decimal-string fields, so we fall back to a cursor scan for totalStaked.
		}}},
	}

	cursor, err := configs.ValidatorsCollections.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate validator stats: %v", err)
	}
	defer cursor.Close(ctx)

	var activeCount, pendingCount, exitedCount, slashedCount int
	for cursor.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		switch row.ID {
		case "active":
			activeCount = row.Count
		case "pending":
			pendingCount = row.Count
		case "exited":
			exitedCount = row.Count
		case "slashed":
			slashedCount = row.Count
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error while reading validator stats: %v", err)
	}

	// Compute total staked via a second aggregation (sum of effectiveBalance).
	// MongoDB $sum works on numeric types; balances are stored as decimal strings,
	// so we convert with $toLong inside the pipeline.
	sumPipeline := mongo.Pipeline{
		bson.D{{Key: "$group", Value: bson.M{
			"_id": nil,
			"totalStaked": bson.M{"$sum": bson.M{"$toLong": "$effectiveBalance"}},
		}}},
	}
	sumCursor, err := configs.ValidatorsCollections.Aggregate(ctx, sumPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate total staked: %v", err)
	}
	defer sumCursor.Close(ctx)

	totalStaked := int64(0)
	if sumCursor.Next(ctx) {
		var sumRow struct {
			TotalStaked int64 `bson:"totalStaked"`
		}
		if err := sumCursor.Decode(&sumRow); err == nil {
			totalStaked = sumRow.TotalStaked
		}
	}

	return &models.ValidatorStatsResponse{
		TotalValidators: int(totalCount),
		ActiveCount:     activeCount,
		PendingCount:    pendingCount,
		ExitedCount:     exitedCount,
		SlashedCount:    slashedCount,
		TotalStaked:     fmt.Sprintf("%d", totalStaked),
		CurrentEpoch:    fmt.Sprintf("%d", currentEpoch),
	}, nil
}
