package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func getChainTiming(ctx context.Context) (models.ChainTiming, error) {
	var timing models.ChainTiming
	if err := configs.SyncStateCollection.FindOne(ctx, bson.M{"_id": models.ChainTimingID}).Decode(&timing); err != nil {
		return timing, fmt.Errorf("chain timing unavailable: %w", err)
	}
	return timing, timing.Validate()
}

func getBeaconEpochInfo(ctx context.Context) (models.EpochInfo, models.ChainTiming, error) {
	var info models.EpochInfo
	timing, err := getChainTiming(ctx)
	if err != nil {
		return info, timing, err
	}
	if err := configs.EpochInfoCollection.FindOne(ctx, bson.M{"_id": "current"}).Decode(&info); err != nil {
		return info, timing, fmt.Errorf("beacon epoch info unavailable: %w", err)
	}
	slot, err := strconv.ParseInt(info.HeadSlot, 10, 64)
	if err != nil {
		return info, timing, fmt.Errorf("invalid beacon head slot")
	}
	epoch, err := timing.EpochAtSlot(slot)
	if err != nil {
		return info, timing, err
	}
	info.HeadEpoch = strconv.FormatInt(epoch, 10)
	return info, timing, nil
}

func currentBeaconEpoch(ctx context.Context) (int64, error) {
	info, _, err := getBeaconEpochInfo(ctx)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(info.HeadEpoch, 10, 64)
}

func BlockStatsWithTiming(ctx context.Context, samples []BlockGasSample) (BlockStats, error) {
	stats := ComputeBlockStats(samples)
	if stats.AvgBlockTimeSec > 0 {
		return stats, nil
	}
	timing, err := getChainTiming(ctx)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return stats, nil
	}
	if err != nil {
		return stats, err
	}
	stats.AvgBlockTimeSec = float64(timing.SecondsPerSlot)
	return stats, nil
}

// BlockTimeSeconds exposes unknown timing as JSON null while preserving gas data.
func (stats BlockStats) BlockTimeSeconds() *float64 {
	if stats.AvgBlockTimeSec <= 0 {
		return nil
	}
	return &stats.AvgBlockTimeSec
}
