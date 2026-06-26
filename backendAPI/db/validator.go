package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
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
		Validators:     validators,
		ValidatorCount: len(validators),
		Epoch:          fmt.Sprintf("%d", currentEpoch),
		TotalStaked:    fmt.Sprintf("%d", totalStaked),
	}, nil
}

// CountValidators returns the total validator count. Uses
// countDocumentsResilient (fast metadata read with an exact-count fallback when
// the metadata reads 0, which it currently does on this deployment).
func CountValidators() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := countDocumentsResilient(ctx, configs.ValidatorsCollections)
	if err != nil {
		return 0, fmt.Errorf("failed to count validators: %v", err)
	}
	return count, nil
}

// HexToInt converts a hex string (with or without 0x prefix) to int64.
// Parses as unsigned first: ParseInt returns 0 for any value with the high
// bit set (block numbers near 2^63), which would silently zero the epoch
// math downstream. Values that overflow int64 are clamped to math.MaxInt64
// so the result stays usable for the /128 epoch division callers perform.
func HexToInt(hexStr string) int64 {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	num, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0
	}
	if num > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(num)
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

// GetEpochs returns a paginated list of epochs from validator_history,
// augmented with finalized/justified status from epoch_info.
func GetEpochs(page, limit int) (*models.EpochsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get current epoch state for finalized/justified status
	var epochInfo models.EpochInfo
	err := configs.EpochInfoCollection.FindOne(ctx, bson.M{"_id": "current"}).Decode(&epochInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch info: %v", err)
	}
	finalizedEpoch := parseEpoch(epochInfo.FinalizedEpoch)
	justifiedEpoch := parseEpoch(epochInfo.JustifiedEpoch)

	// Count total epoch records for "total pages" pagination. Uses
	// countDocumentsResilient: the fast metadata read collapses to 0 on this
	// deployment, which would wrongly show a single page.
	total, err := countDocumentsResilient(ctx, configs.ValidatorHistoryCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to count epochs: %v", err)
	}

	// Query with pagination, sorted newest first by numeric epoch
	skip := int64((page - 1) * limit)
	findOpts := options.Find().
		SetSort(bson.D{{Key: "epochInt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := configs.ValidatorHistoryCollection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to query epochs: %v", err)
	}
	defer cursor.Close(ctx)

	var records []models.ValidatorHistoryRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode epochs: %v", err)
	}

	epochs := make([]models.EpochListItem, 0, len(records))
	for _, r := range records {
		epochNum := parseEpoch(r.Epoch)
		var status string
		if epochNum <= finalizedEpoch {
			status = "finalized"
		} else if epochNum <= justifiedEpoch {
			status = "justified"
		} else {
			status = "pending"
		}

		epochs = append(epochs, models.EpochListItem{
			Epoch:           r.Epoch,
			Timestamp:       r.Timestamp,
			Status:          status,
			ValidatorsCount: r.ValidatorsCount,
			ActiveCount:     r.ActiveCount,
			TotalStaked:     r.TotalStaked,
		})
	}

	return &models.EpochsResponse{
		Epochs:         epochs,
		Total:          total,
		FinalizedEpoch: epochInfo.FinalizedEpoch,
		JustifiedEpoch: epochInfo.JustifiedEpoch,
	}, nil
}

// GetEpochDetail returns full epoch information including all slots (proposed/missed).
func GetEpochDetail(epochId string) (*models.EpochDetailResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	epochNum, err := strconv.ParseInt(epochId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid epoch number: %v", err)
	}

	startSlot := epochNum * SlotsPerEpoch
	endSlot := (epochNum + 1) * SlotsPerEpoch

	// Get epoch_info for chain head status
	var epochInfo models.EpochInfo
	err = configs.EpochInfoCollection.FindOne(ctx, bson.M{"_id": "current"}).Decode(&epochInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch info: %v", err)
	}

	headEpoch := parseEpoch(epochInfo.HeadEpoch)
	if epochNum > headEpoch {
		return nil, fmt.Errorf("epoch %d has not yet occurred (head: %d)", epochNum, headEpoch)
	}

	finalizedEpoch := parseEpoch(epochInfo.FinalizedEpoch)
	justifiedEpoch := parseEpoch(epochInfo.JustifiedEpoch)

	var status string
	if epochNum <= finalizedEpoch {
		status = "finalized"
	} else if epochNum <= justifiedEpoch {
		status = "justified"
	} else {
		status = "pending"
	}

	// Get validator_history record (may not exist for very recent epochs)
	var historyRecord models.ValidatorHistoryRecord
	_ = configs.ValidatorHistoryCollection.FindOne(ctx, bson.M{"epoch": epochId}).Decode(&historyRecord)

	// Get blocks in this epoch's slot range
	blockFilter := bson.M{
		"blockNumberInt": bson.M{"$gte": startSlot, "$lt": endSlot},
	}
	projection := bson.D{
		{Key: "blockNumberInt", Value: 1},
		{Key: "result.number", Value: 1},
		{Key: "result.timestamp", Value: 1},
		{Key: "result.miner", Value: 1},
		{Key: "result.transactions", Value: 1},
		{Key: "result.gasUsed", Value: 1},
		{Key: "result.hash", Value: 1},
	}
	findOpts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "blockNumberInt", Value: 1}})

	cursor, err := configs.BlocksCollection.Find(ctx, blockFilter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %v", err)
	}
	defer cursor.Close(ctx)

	blockMap := make(map[int64]models.EpochDetailBlock)
	for cursor.Next(ctx) {
		var doc struct {
			BlockNumberInt int64         `bson:"blockNumberInt"`
			Result         models.Result `bson:"result"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		blockMap[doc.BlockNumberInt] = models.EpochDetailBlock{
			Slot:         doc.BlockNumberInt,
			Status:       "proposed",
			Timestamp:    doc.Result.Timestamp,
			Proposer:     doc.Result.Miner,
			Transactions: len(doc.Result.Transactions),
			GasUsed:      doc.Result.GasUsed,
			BlockHash:    doc.Result.Hash,
		}
	}

	// Build full 128-slot list
	blocks := make([]models.EpochDetailBlock, 0, SlotsPerEpoch)
	proposedCount := 0
	missedCount := 0
	for slot := startSlot; slot < endSlot; slot++ {
		if b, ok := blockMap[slot]; ok {
			blocks = append(blocks, b)
			proposedCount++
		} else {
			blocks = append(blocks, models.EpochDetailBlock{
				Slot:   slot,
				Status: "missed",
			})
			missedCount++
		}
	}

	return &models.EpochDetailResponse{
		Epoch:           epochId,
		Timestamp:       historyRecord.Timestamp,
		Status:          status,
		ValidatorsCount: historyRecord.ValidatorsCount,
		ActiveCount:     historyRecord.ActiveCount,
		PendingCount:    historyRecord.PendingCount,
		ExitedCount:     historyRecord.ExitedCount,
		SlashedCount:    historyRecord.SlashedCount,
		TotalStaked:     historyRecord.TotalStaked,
		StartSlot:       startSlot,
		EndSlot:         endSlot,
		Blocks:          blocks,
		FinalizedEpoch:  epochInfo.FinalizedEpoch,
		JustifiedEpoch:  epochInfo.JustifiedEpoch,
		HeadEpoch:       epochInfo.HeadEpoch,
		ProposedCount:   proposedCount,
		MissedCount:     missedCount,
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

	// Check whether the collection has any documents at all. Uses
	// countDocumentsResilient because the EstimatedDocumentCount metadata reads
	// 0 for populated collections on this deployment, which would wrongly short
	// circuit to an empty stats response below.
	totalCount, err := countDocumentsResilient(ctx, configs.ValidatorsCollections)
	if err != nil {
		return nil, fmt.Errorf("failed to count validators: %v", err)
	}
	if totalCount == 0 {
		return &models.ValidatorStatsResponse{
			CurrentEpoch: fmt.Sprintf("%d", currentEpoch),
		}, nil
	}

	// Use a single aggregation to compute per-status counts and total staked.
	// Status computation requires knowing currentEpoch, which MongoDB doesn't know,
	// so we project the fields needed and compute buckets in a $group stage using
	// $cond expressions that mirror getValidatorStatus logic. A $facet then fans
	// the same scanned documents into the status buckets and the balance sum so
	// the whole thing is one cursor over the collection instead of two passes.
	// $sum needs numeric input and balances are decimal strings, so the sum
	// branch converts with $toLong inside the pipeline.
	currentEpochStr := fmt.Sprintf("%d", currentEpoch)

	pipeline := mongo.Pipeline{
		// Add a computed "status" field using the same rules as getValidatorStatus.
		bson.D{{Key: "$addFields", Value: bson.M{
			"_computedStatus": bson.M{
				"$switch": bson.M{
					"branches": []bson.M{
						{
							// slashed
							"case": bson.M{"$eq": []interface{}{"$slashed", true}},
							"then": "slashed",
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
		bson.D{{Key: "$facet", Value: bson.M{
			"statusCounts": []bson.M{
				{"$group": bson.M{
					"_id":   "$_computedStatus",
					"count": bson.M{"$sum": 1},
				}},
			},
			"totals": []bson.M{
				{"$group": bson.M{
					"_id":         nil,
					"totalStaked": bson.M{"$sum": bson.M{"$toLong": "$effectiveBalance"}},
				}},
			},
		}}},
	}

	cursor, err := configs.ValidatorsCollections.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate validator stats: %v", err)
	}
	defer cursor.Close(ctx)

	var facetResult struct {
		StatusCounts []struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		} `bson:"statusCounts"`
		Totals []struct {
			TotalStaked int64 `bson:"totalStaked"`
		} `bson:"totals"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&facetResult); err != nil {
			return nil, fmt.Errorf("failed to decode validator stats: %v", err)
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error while reading validator stats: %v", err)
	}

	var activeCount, pendingCount, exitedCount, slashedCount int
	for _, row := range facetResult.StatusCounts {
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

	totalStaked := int64(0)
	if len(facetResult.Totals) > 0 {
		totalStaked = facetResult.Totals[0].TotalStaked
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
