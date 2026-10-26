package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type epochSlotAssignment struct {
	Epoch          int64  `bson:"epoch"`
	Slot           int64  `bson:"slot"`
	ValidatorIndex string `bson:"validatorIndex"`
	Pubkey         string `bson:"pubkey"`
}

type epochExecutionBlock struct {
	BlockNumberInt int64         `bson:"blockNumberInt"`
	Result         models.Result `bson:"result"`
}

func parseSlotTimestamp(raw string) (int64, error) {
	base := 10
	if strings.HasPrefix(raw, "0x") {
		base = 16
		raw = raw[2:]
	}
	return strconv.ParseInt(raw, base, 64)
}

// epochTimestampFilter uses the stored beacon clock for each slot timestamp.
// Execution heights remain independent of slots across missed proposals.
func epochTimestampFilter(start, end int64, timing models.ChainTiming) (bson.M, error) {
	genesis := timing.GenesisTime
	if err := timing.Validate(); err != nil {
		return nil, err
	}
	if genesis <= 0 || start < 0 || end <= start || end-start != timing.SlotsPerEpoch || end > (math.MaxInt64-genesis)/timing.SecondsPerSlot {
		return nil, fmt.Errorf("invalid epoch slot clock")
	}
	timestamps := make([]string, 0, timing.SlotsPerEpoch*2)
	for slot := start; slot < end; slot++ {
		timestamp := genesis + slot*timing.SecondsPerSlot
		timestamps = append(timestamps, fmt.Sprintf("0x%x", timestamp), strconv.FormatInt(timestamp, 10))
	}
	return bson.M{"result.timestamp": bson.M{"$in": timestamps}, "ingestionState": completedBlockIngestionState}, nil
}

func mapEpochSlots(epoch int64, timing models.ChainTiming, blocks []epochExecutionBlock, assignments []epochSlotAssignment) ([]models.EpochDetailBlock, int, error) {
	start, end, err := timing.EpochSlots(epoch)
	if err != nil {
		return nil, 0, err
	}
	if _, err := epochTimestampFilter(start, end, timing); err != nil {
		return nil, 0, err
	}
	rows := make([]models.EpochDetailBlock, timing.SlotsPerEpoch)
	for i := range rows {
		rows[i] = models.EpochDetailBlock{Slot: start + int64(i), Status: "missed"}
	}
	seenAssignments := make(map[int64]bool)
	for _, assignment := range assignments {
		if assignment.Epoch != epoch || assignment.Slot < start || assignment.Slot >= end || seenAssignments[assignment.Slot] {
			return nil, 0, fmt.Errorf("invalid slot assignment data")
		}
		seenAssignments[assignment.Slot] = true
		row := &rows[assignment.Slot-start]
		row.ProposerIndex = assignment.ValidatorIndex
		row.ProposerPubkey = assignment.Pubkey
	}
	proposed := 0
	for _, block := range blocks {
		timestamp, err := parseSlotTimestamp(block.Result.Timestamp)
		if err != nil || timestamp < timing.GenesisTime || (timestamp-timing.GenesisTime)%timing.SecondsPerSlot != 0 {
			return nil, 0, fmt.Errorf("invalid slot timestamp in indexed block")
		}
		slot := (timestamp - timing.GenesisTime) / timing.SecondsPerSlot
		if slot < start || slot >= end || block.BlockNumberInt < 0 || block.Result.Hash == "" {
			return nil, 0, fmt.Errorf("invalid block in epoch slot range")
		}
		row := &rows[slot-start]
		if row.Status == "proposed" {
			return nil, 0, fmt.Errorf("multiple indexed blocks for slot %d", slot)
		}
		row.Status = "proposed"
		row.Timestamp = block.Result.Timestamp
		row.BlockNumber = strconv.FormatInt(block.BlockNumberInt, 10)
		row.BlockHash = block.Result.Hash
		row.Transactions = len(block.Result.Transactions)
		row.GasUsed = block.Result.GasUsed
		row.Proposer = block.Result.Miner // Legacy execution coinbase field.
		proposed++
	}
	return rows, proposed, nil
}

func getEpochSlots(ctx context.Context, epoch int64, timing models.ChainTiming) ([]models.EpochDetailBlock, int, error) {
	start, end, err := timing.EpochSlots(epoch)
	if err != nil {
		return nil, 0, err
	}

	filter, err := epochTimestampFilter(start, end, timing)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := configs.BlocksCollection.Find(ctx, filter, options.Find().SetProjection(bson.M{
		"blockNumberInt": 1, "result.timestamp": 1, "result.miner": 1,
		"result.transactions": 1, "result.gasused": 1, "result.hash": 1,
	}))
	if err != nil {
		return nil, 0, fmt.Errorf("query epoch blocks: %w", err)
	}
	defer cursor.Close(ctx)
	var blocks []epochExecutionBlock
	if err := cursor.All(ctx, &blocks); err != nil {
		return nil, 0, fmt.Errorf("decode epoch blocks: %w", err)
	}
	cursor, err = configs.SlotAssignmentsCollection.Find(ctx, bson.M{"epoch": epoch})
	if err != nil {
		return nil, 0, fmt.Errorf("query slot assignments: %w", err)
	}
	defer cursor.Close(ctx)
	var assignments []epochSlotAssignment
	if err := cursor.All(ctx, &assignments); err != nil {
		return nil, 0, fmt.Errorf("decode slot assignments: %w", err)
	}
	return mapEpochSlots(epoch, timing, blocks, assignments)
}
