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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get latest block
	latestBlock := GetLatestBlockFromDB()
	if latestBlock == nil {
		return
	}

	// Calculate volume for the last 24 hours
	currentBlockTimestamp := latestBlock.Result.Timestamp
	targetBlockTimestamp := utils.SubtractHexNumbers(currentBlockTimestamp, "0x15180") // 0x15180 = 86400 (24 hours in seconds)

	// Get all transfers in the last 24 hours
	cursor, err := configs.TransferCollections.Find(ctx, bson.M{})
	if err != nil {
		configs.Logger.Info("Failed to find transfers", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	// Calculate total volume
	totalVolume := "0x0"
	for cursor.Next(ctx) {
		var singleTransfer models.Transfer
		if err = cursor.Decode(&singleTransfer); err != nil {
			continue
		}

		// Check if transfer is within time range
		if utils.CompareHexNumbers(singleTransfer.BlockTimestamp, targetBlockTimestamp) >= 0 &&
			utils.CompareHexNumbers(singleTransfer.BlockTimestamp, currentBlockTimestamp) <= 0 {
			totalVolume = utils.AddHexNumbers(totalVolume, singleTransfer.Value)
		}
	}

	// Convert hex volume to int64
	volumeInt := utils.HexToInt(totalVolume)
	var volume int64
	if !volumeInt.IsInt64() {
		configs.Logger.Warn("Volume exceeds int64 range, capping at max int64",
			zap.String("volume_hex", totalVolume))
		volume = 9223372036854775807 // max int64
	} else {
		volume = volumeInt.Int64()
	}

	// Convert timestamp to int64
	timestampInt := utils.HexToInt(currentBlockTimestamp)
	var timestamp int64
	if !timestampInt.IsInt64() {
		configs.Logger.Warn("Timestamp exceeds int64 range, using current time",
			zap.String("timestamp_hex", currentBlockTimestamp))
		timestamp = time.Now().Unix()
	} else {
		timestamp = timestampInt.Int64()
	}

	// Update volume in database
	filter := bson.M{"type": "daily_volume"}
	update := bson.M{
		"$set": bson.M{
			"volume":    volume,
			"timestamp": timestamp,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err = configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update volume",
			zap.Error(err),
			zap.Int64("volume", volume),
			zap.Int64("timestamp", timestamp))
	} else {
		configs.Logger.Info("Successfully updated volume",
			zap.Int64("volume", volume),
			zap.Int64("timestamp", timestamp))
	}
}

func GetVolumeFromBlockTimestamps(startTimestamp string, endTimestamp string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := configs.TransferCollections.Find(ctx, bson.M{})
	if err != nil {
		configs.Logger.Info("Failed to find transfers", zap.Error(err))
		return "0x0"
	}
	defer cursor.Close(ctx)

	totalVolume := "0x0"
	for cursor.Next(ctx) {
		var singleTransfer models.Transfer
		if err = cursor.Decode(&singleTransfer); err != nil {
			continue
		}

		// Check if transfer is within time range
		if utils.CompareHexNumbers(singleTransfer.BlockTimestamp, startTimestamp) >= 0 &&
			utils.CompareHexNumbers(singleTransfer.BlockTimestamp, endTimestamp) <= 0 {
			totalVolume = utils.AddHexNumbers(totalVolume, singleTransfer.Value)
		}
	}

	return totalVolume
}
