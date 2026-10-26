package models

import (
	"fmt"
	"math"
)

const ChainTimingID = "chain_timing"

// ChainTiming is the beacon clock persisted in sync_state at startup.
type ChainTiming struct {
	SlotsPerEpoch  int64 `bson:"slotsPerEpoch" json:"slotsPerEpoch"`
	SecondsPerSlot int64 `bson:"secondsPerSlot" json:"secondsPerSlot"`
	GenesisTime    int64 `bson:"genesisTime" json:"genesisTime"`
}

func (t ChainTiming) Validate() error {
	// Bound allocations for epoch tables and reject overflowing clocks.
	if t.SlotsPerEpoch <= 0 || t.SlotsPerEpoch > 65536 || t.SecondsPerSlot <= 0 || t.GenesisTime <= 0 || t.SecondsPerSlot > (math.MaxInt64-t.GenesisTime)/t.SlotsPerEpoch {
		return fmt.Errorf("invalid beacon chain timing")
	}
	return nil
}

func (t ChainTiming) EpochAtSlot(slot int64) (int64, error) {
	if err := t.Validate(); err != nil {
		return 0, err
	}
	if slot < 0 {
		return 0, fmt.Errorf("invalid beacon slot")
	}
	return slot / t.SlotsPerEpoch, nil
}

// EpochSlots returns the inclusive first slot and exclusive end slot.
func (t ChainTiming) EpochSlots(epoch int64) (int64, int64, error) {
	if err := t.Validate(); err != nil {
		return 0, 0, err
	}
	if epoch < 0 || epoch >= math.MaxInt64/t.SlotsPerEpoch {
		return 0, 0, fmt.Errorf("invalid epoch number")
	}
	start, end := epoch*t.SlotsPerEpoch, (epoch+1)*t.SlotsPerEpoch
	if end > (math.MaxInt64-t.GenesisTime)/t.SecondsPerSlot {
		return 0, 0, fmt.Errorf("epoch timestamp overflow")
	}
	return start, end, nil
}

func (t ChainTiming) EpochTimestamp(epoch int64) (int64, error) {
	slot, _, err := t.EpochSlots(epoch)
	if err != nil {
		return 0, err
	}
	return t.GenesisTime + slot*t.SecondsPerSlot, nil
}
