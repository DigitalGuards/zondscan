package db

import (
	"backendAPI/configs"
	"backendAPI/hexutil"
	"backendAPI/models"
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ReturnValidators returns all validators with computed status and totals.
// It queries the per-document validators collection directly instead of loading
// a single mega-document and iterating in Go.
func ReturnValidators(pageToken string) (*models.ValidatorResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Read the beacon head once for validator lifecycle calculations.
	currentEpoch, err := currentBeaconEpoch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get beacon epoch: %v", err)
	}

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
			Index:                    d.ID,
			Address:                  d.PublicKeyHex,
			WithdrawalCredentialsHex: d.WithdrawalCredentialsHex,
			Status:                   status,
			Age:                      age,
			StakedAmount:             d.EffectiveBalance,
			IsActive:                 isActive,
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

// CountValidators returns the exact total validator count using the _id_ index.
func CountValidators() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := countDocumentsResilient(ctx, configs.ValidatorsCollections)
	if err != nil {
		return 0, fmt.Errorf("failed to count validators: %v", err)
	}
	return count, nil
}

// HexToInt preserves the legacy numeric parser's range and error semantics.
func HexToInt(hexStr string) int64 {
	num, err := hexutil.ParseInt64(hexStr)
	if err != nil && !errors.Is(err, hexutil.ErrRange) {
		return 0
	}
	return num
}

// FAR_FUTURE_EPOCH represents a validator that hasn't exited.
const FAR_FUTURE_EPOCH = "18446744073709551615"

var ErrValidatorNotFound = errors.New("validator not found")

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

	epochInfo, timing, err := getBeaconEpochInfo(ctx)
	if err != nil {
		return nil, err
	}

	headSlot := parseEpoch(epochInfo.HeadSlot)
	slotInEpoch := headSlot % timing.SlotsPerEpoch
	slotsRemaining := timing.SlotsPerEpoch - slotInEpoch
	timeToNextEpoch := slotsRemaining * timing.SecondsPerSlot

	return &models.EpochInfoResponse{
		GenesisTime:     timing.GenesisTime,
		HeadEpoch:       epochInfo.HeadEpoch,
		HeadSlot:        epochInfo.HeadSlot,
		FinalizedEpoch:  epochInfo.FinalizedEpoch,
		JustifiedEpoch:  epochInfo.JustifiedEpoch,
		SlotsPerEpoch:   timing.SlotsPerEpoch,
		SecondsPerSlot:  timing.SecondsPerSlot,
		SlotInEpoch:     slotInEpoch,
		TimeToNextEpoch: timeToNextEpoch,
		UpdatedAt:       epochInfo.UpdatedAt,
	}, nil
}

// GetValidatorHistory retrieves historical validator data.
func GetValidatorHistory(limit int) (*models.ValidatorHistoryResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cap an unbounded request so a limit=0 (or negative) caller can't force a
	// full-collection scan of validator_history. Explicit small limits are
	// honoured unchanged.
	if limit <= 0 {
		limit = 1000
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "epochInt", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := configs.ValidatorHistoryCollection.Find(ctx, beaconHistoryFilter(), findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator history: %v", err)
	}
	defer cursor.Close(ctx)

	history := make([]models.ValidatorHistoryRecord, 0)
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

	currentEpoch, err := currentBeaconEpoch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get beacon epoch: %v", err)
	}

	// Try lookup by _id (index) first, then fall back to publicKeyHex.
	filter := bson.M{"$or": []bson.M{
		{"_id": id},
		{"publicKeyHex": id},
	}}

	var doc models.ValidatorDocument
	err = configs.ValidatorsCollections.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrValidatorNotFound
		}
		return nil, fmt.Errorf("failed to get validator: %v", err)
	}

	status := getValidatorStatus(doc.ActivationEpoch, doc.ExitEpoch, doc.Slashed, currentEpoch)
	activationEpoch := parseEpoch(doc.ActivationEpoch)
	age := int64(0)
	if activationEpoch <= currentEpoch {
		age = currentEpoch - activationEpoch
	}

	query := models.ValidatorActivityQuery{Page: 1, Limit: 20, ValidatorIndex: doc.ID}
	events, eventTotal, err := activityPage[models.SlashingEvent](ctx, configs.SlashingEventsCollection, query, true)
	if err != nil {
		return nil, fmt.Errorf("read validator slashings: %w", err)
	}
	changes, changeTotal, err := activityPage[models.ValidatorSetChange](ctx, configs.ValidatorSetChangesCollection, query, false)
	if err != nil {
		return nil, fmt.Errorf("read validator changes: %w", err)
	}
	return &models.ValidatorDetailResponse{
		SlashingEvents: events, SlashingEventsTotal: eventTotal,
		ValidatorSetChanges: changes, ValidatorSetChangesTotal: changeTotal,
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
	epochInfo, _, err := getBeaconEpochInfo(ctx)
	if err != nil {
		return nil, err
	}

	finalizedEpoch := parseEpoch(epochInfo.FinalizedEpoch)
	justifiedEpoch := parseEpoch(epochInfo.JustifiedEpoch)

	// Count total epoch records exactly for "total pages" pagination.
	total, err := configs.ValidatorHistoryCollection.CountDocuments(ctx, beaconHistoryFilter())
	if err != nil {
		return nil, fmt.Errorf("failed to count epochs: %v", err)
	}

	// Query with pagination, sorted newest first by numeric epoch
	skip := int64((page - 1) * limit)
	findOpts := options.Find().
		SetSort(bson.D{{Key: "epochInt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := configs.ValidatorHistoryCollection.Find(ctx, beaconHistoryFilter(), findOpts)
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

	epochInfo, timing, err := getBeaconEpochInfo(ctx)
	if err != nil {
		return nil, err
	}
	startSlot, endSlot, err := timing.EpochSlots(epochNum)
	if err != nil {
		return nil, err
	}
	epochId = strconv.FormatInt(epochNum, 10)

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
	filter := beaconHistoryFilter()
	filter["epoch"] = epochId
	err = configs.ValidatorHistoryCollection.FindOne(ctx, filter).Decode(&historyRecord)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("read epoch history: %w", err)
	}

	blocks, proposedCount, err := getEpochSlots(ctx, epochNum, timing)
	if err != nil {
		return nil, err
	}
	missedCount := len(blocks) - proposedCount

	timestamp, err := timing.EpochTimestamp(epochNum)
	if err != nil {
		return nil, err
	}
	result := &models.EpochDetailResponse{
		Epoch: epochId, Timestamp: timestamp, Status: status,
		StartSlot:      startSlot,
		EndSlot:        endSlot,
		Blocks:         blocks,
		FinalizedEpoch: epochInfo.FinalizedEpoch,
		JustifiedEpoch: epochInfo.JustifiedEpoch,
		HeadEpoch:      epochInfo.HeadEpoch,
		ProposedCount:  proposedCount,
		MissedCount:    missedCount,
	}
	applyEpochSummary(result, historyRecord)
	return result, nil
}

// GetValidatorStats returns aggregated validator statistics using a MongoDB aggregation
// pipeline instead of loading all validators into Go memory.
func GetValidatorStats() (*models.ValidatorStatsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	currentEpoch, err := currentBeaconEpoch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get beacon epoch: %v", err)
	}

	// Check whether the collection has any documents using an exact count;
	// metadata estimates can be stale for populated collections.
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
							"case": bson.M{"$gt": []interface{}{bson.M{"$toDecimal": "$activationEpoch"}, currentEpoch}},
							"then": "pending",
						},
						{
							// exited: exitEpoch <= currentEpoch AND exitEpoch != FAR_FUTURE_EPOCH
							"case": bson.M{"$and": []bson.M{
								{"$lte": []interface{}{bson.M{"$toDecimal": "$exitEpoch"}, currentEpoch}},
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

func beaconHistoryFilter() bson.M { return bson.M{"source": models.BeaconStateHistorySource} }

func applyEpochSummary(result *models.EpochDetailResponse, record models.ValidatorHistoryRecord) {
	if record.Source != models.BeaconStateHistorySource {
		return
	}
	result.SummaryAvailable = true
	result.ValidatorsCount = &record.ValidatorsCount
	result.ActiveCount = &record.ActiveCount
	result.PendingCount = &record.PendingCount
	result.ExitedCount = &record.ExitedCount
	result.SlashedCount = &record.SlashedCount
	result.TotalStaked = &record.TotalStaked
}
