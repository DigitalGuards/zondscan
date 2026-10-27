package db

import (
	"backendAPI/models"
	"fmt"
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestEpochSlotsMapExecutionTimestampsAcrossMissedSlots(t *testing.T) {
	const genesis = int64(1700000000)
	blocks := []epochExecutionBlock{
		{BlockNumberInt: 9, Result: models.Result{Timestamp: fmt.Sprintf("0x%x", genesis+16*60), Hash: "0xaaa", GasUsed: "0x5208"}},
		{BlockNumberInt: 10, Result: models.Result{Timestamp: fmt.Sprintf("%d", genesis+18*60), Hash: "0xbbb", Transactions: []models.Transaction{{Hash: "0xccc"}}}},
	}
	assignments := []epochSlotAssignment{
		{Epoch: 2, Slot: 16, ValidatorIndex: "0", Pubkey: "0xaabb"},
		{Epoch: 2, Slot: 17, ValidatorIndex: "42", Pubkey: "0xccdd"},
	}
	rows, proposed, err := mapEpochSlots(2, slotTestTiming(genesis), blocks, assignments)
	if err != nil || len(rows) != 8 || proposed != 2 {
		t.Fatalf("rows=%v proposed=%d error=%v", rows, proposed, err)
	}
	if rows[0].Slot != 16 || rows[0].BlockNumber != "9" || rows[0].ProposerIndex != "0" || rows[0].GasUsed != "0x5208" {
		t.Fatalf("first slot = %+v", rows[0])
	}
	if row := rows[1]; row.Slot != 17 || row.Status != "missed" || row.ProposerIndex != "42" || row.ProposerPubkey != "0xccdd" || row.BlockNumber != "" || row.BlockHash != "" {
		t.Fatalf("missed slot = %+v", row)
	}
	if row := rows[2]; row.Slot != 18 || row.Status != "proposed" || row.BlockNumber != "10" || row.BlockHash != "0xbbb" || row.Transactions != 1 || row.ProposerIndex != "" {
		t.Fatalf("next proposed slot = %+v", row)
	}
	if row := rows[7]; row.Status != "missed" || row.ProposerIndex != "" {
		t.Fatalf("unknown slot = %+v", row)
	}
}

func TestEpochQueryUsesSlotTimestampsAndCompleteBlocks(t *testing.T) {
	filter, err := epochTimestampFilter(16, 24, slotTestTiming(1700000000))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := filter["blockNumberInt"]; exists {
		t.Fatal("query uses execution height as a slot")
	}
	if filter["ingestionState"] != completedBlockIngestionState {
		t.Fatal("query exposes incomplete blocks")
	}
	timestamps := filter["result.timestamp"].(bson.M)["$in"].([]string)
	if len(timestamps) != 16 || timestamps[0] != fmt.Sprintf("0x%x", 1700000000+16*60) || timestamps[15] != fmt.Sprint(1700000000+23*60) {
		t.Fatalf("timestamps = %v", timestamps)
	}
}

func TestEpochMappingRejectsDuplicateAndUnalignedBlocks(t *testing.T) {
	const genesis = int64(1700000000)
	valid := epochExecutionBlock{BlockNumberInt: 1, Result: models.Result{Timestamp: fmt.Sprint(genesis + 8*60), Hash: "0xabc"}}
	if _, _, err := mapEpochSlots(1, slotTestTiming(genesis), []epochExecutionBlock{valid, valid}, nil); err == nil {
		t.Fatal("duplicate slot accepted")
	}
	for _, timestamp := range []string{"invalid", fmt.Sprint(genesis - 1), fmt.Sprint(genesis + 8*60 + 1), fmt.Sprint(genesis + 16*60)} {
		block := valid
		block.Result.Timestamp = timestamp
		if _, _, err := mapEpochSlots(1, slotTestTiming(genesis), []epochExecutionBlock{block}, nil); err == nil {
			t.Fatalf("timestamp %s accepted", timestamp)
		}
	}
	if _, _, err := mapEpochSlots(1, slotTestTiming(genesis), nil, []epochSlotAssignment{{Epoch: 1, Slot: 16}}); err == nil {
		t.Fatal("out-of-range assignment accepted")
	}
}

func TestEpochSlotBoundsAndMissingClock(t *testing.T) {
	for _, epoch := range []int64{-1, math.MaxInt64, math.MaxInt64 / 8} {
		if _, _, err := slotTestTiming(1700000000).EpochSlots(epoch); err == nil {
			t.Fatalf("epoch %d accepted", epoch)
		}
	}
	if _, _, err := mapEpochSlots(1, slotTestTiming(0), nil, nil); err == nil {
		t.Fatal("missing beacon genesis accepted")
	}
	if _, err := epochTimestampFilter(8, 16, slotTestTiming(math.MaxInt64-1)); err == nil {
		t.Fatal("timestamp overflow accepted")
	}
}

func slotTestTiming(genesis int64) models.ChainTiming {
	return models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: genesis}
}
