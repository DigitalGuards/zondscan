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

	// Update volume in database
	filter := bson.M{"type": "daily_volume"}
	update := bson.M{
		"$set": bson.M{
			"volume":    totalVolume,
			"timestamp": currentBlockTimestamp,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err = configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update volume",
			zap.Error(err),
			zap.String("volume", totalVolume),
			zap.String("timestamp", currentBlockTimestamp))
	} else {
		configs.Logger.Info("Successfully updated volume",
			zap.String("volume", totalVolume),
			zap.String("timestamp", currentBlockTimestamp))
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
