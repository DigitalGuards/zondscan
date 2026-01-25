package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func GetDailyTransactionVolume() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get latest synced block number from sync_state (more reliable than sorting hex strings)
	latestBlockNumber := GetLastKnownBlockNumber()
	if latestBlockNumber == "" || latestBlockNumber == "0x0" {
		configs.Logger.Warn("No latest block found for volume calculation")
		return
	}

	// Fetch the block to get its timestamp
	latestBlock := GetBlockFromDB(latestBlockNumber)
	if latestBlock == nil {
		configs.Logger.Warn("Could not fetch latest block for volume calculation",
			zap.String("blockNumber", latestBlockNumber))
		return
	}

	// Calculate volume for the last 24 hours
	currentBlockTimestamp := latestBlock.Result.Timestamp
	targetBlockTimestamp := utils.SubtractHexNumbers(currentBlockTimestamp, "0x15180") // 0x15180 = 86400 (24 hours in seconds)

	// Query transfers within the time range using MongoDB filter
	filter := bson.M{
		"blockTimestamp": bson.M{
			"$gte": targetBlockTimestamp,
			"$lte": currentBlockTimestamp,
		},
	}

	cursor, err := configs.TransferCollections.Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to find transfers", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	// Calculate total volume (values are stored as float64 in QRL)
	var totalVolume float64 = 0
	var transferCount int = 0
	for cursor.Next(ctx) {
		var singleTransfer models.Transfer
		if err = cursor.Decode(&singleTransfer); err != nil {
			configs.Logger.Debug("Failed to decode transfer", zap.Error(err))
			continue
		}
		totalVolume += singleTransfer.Value
		transferCount++
	}

	configs.Logger.Info("Calculated daily volume",
		zap.Float64("totalVolume", totalVolume),
		zap.Int("transferCount", transferCount),
		zap.String("fromTimestamp", targetBlockTimestamp),
		zap.String("toTimestamp", currentBlockTimestamp))

	// Update volume in database (store as float64 for precision)
	filter = bson.M{"type": "daily_volume"}
	update := bson.M{
		"$set": bson.M{
			"volume":        totalVolume,
			"timestamp":     currentBlockTimestamp,
			"transferCount": transferCount,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err = configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update volume",
			zap.Error(err),
			zap.Float64("volume", totalVolume),
			zap.String("timestamp", currentBlockTimestamp))
	} else {
		configs.Logger.Info("Successfully updated volume",
			zap.Float64("volume", totalVolume),
			zap.String("timestamp", currentBlockTimestamp))
	}
}

func GetVolumeFromBlockTimestamps(startTimestamp string, endTimestamp string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query transfers within the time range using MongoDB filter
	filter := bson.M{
		"blockTimestamp": bson.M{
			"$gte": startTimestamp,
			"$lte": endTimestamp,
		},
	}

	cursor, err := configs.TransferCollections.Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to find transfers", zap.Error(err))
		return 0
	}
	defer cursor.Close(ctx)

	var totalVolume float64 = 0
	for cursor.Next(ctx) {
		var singleTransfer models.Transfer
		if err = cursor.Decode(&singleTransfer); err != nil {
			continue
		}
		totalVolume += singleTransfer.Value
	}

	return totalVolume
}
