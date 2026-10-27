package models

import (
	"math"
	"testing"
)

func TestChainTimingEpochBoundaries(t *testing.T) {
	timing := ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1790582760}
	for _, tc := range []struct{ slot, epoch int64 }{{0, 0}, {7, 0}, {8, 1}, {39999, 4999}, {40000, 5000}} {
		epoch, err := timing.EpochAtSlot(tc.slot)
		if err != nil || epoch != tc.epoch {
			t.Fatalf("slot %d = %d, %v", tc.slot, epoch, err)
		}
	}
	timestamp, err := timing.EpochTimestamp(5000)
	if err != nil || timestamp != 1792982760 {
		t.Fatalf("timestamp=%d error=%v", timestamp, err)
	}
	for _, epoch := range []int64{-1, math.MaxInt64, math.MaxInt64 / 8} {
		if _, _, err := timing.EpochSlots(epoch); err == nil {
			t.Fatal("invalid epoch accepted")
		}
	}
	if _, err := (ChainTiming{}).EpochAtSlot(0); err == nil {
		t.Fatal("missing timing accepted")
	}
}
