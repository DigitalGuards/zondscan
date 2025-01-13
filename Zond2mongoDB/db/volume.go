package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func CalculateAndStoreAverageVolume() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var targetBlockTimestamp uint64

	latestBlock := GetLatestBlockNumber()
	currentBlockTimestamp := latestBlock.Timestamp
	targetBlockTimestamp = uint64(time.Now().AddDate(0, 0, -1).Unix())

	fmt.Println(targetBlockTimestamp)
	fmt.Println(currentBlockTimestamp)
	fmt.Println(latestBlock.Number)

	volume := GetVolumeFromBlockTimestamps(targetBlockTimestamp, currentBlockTimestamp, latestBlock.Number)

	// Store volume directly as int64 since we're already working with the correct units
	update := primitive.D{
		{Key: "$set", Value: primitive.D{
			{Key: "volume", Value: volume},
		}},
	}

	updateResult, err := configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, primitive.D{}, update, options.Update().SetUpsert(true))
	if err != nil {
		configs.Logger.Warn("Failed to update daily transaction volume: ", zap.Error(err))
	} else {
		configs.Logger.Info("Update Result: ", zap.Any("UpdateResult", updateResult))
	}
}

func GetVolumeFromBlockTimestamps(targetBlockTimestamp uint64, currentBlockTimestamp uint64, latestBlockNumber uint64) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var volume int64
	defer cancel()

	projection := primitive.D{
		{Key: "blockNumber", Value: 1},
		{Key: "blockTimestamp", Value: 1},
		{Key: "value", Value: 1},
	}
	sortOpt := primitive.D{{Key: "blockNumber", Value: -1}}
	options := options.Find().SetProjection(projection).SetSort(sortOpt)

	results, err := configs.TransferCollections.Find(ctx, primitive.D{}, options)

	if err != nil {
		configs.Logger.Info("Error finding documents in Transfer collections", zap.Error(err))
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransfer models.Transfer
		if err = results.Decode(&singleTransfer); err != nil {
			configs.Logger.Info("Error decoding Transfer:", zap.Error(err))
		}

		if singleTransfer.BlockTimestamp >= targetBlockTimestamp {
			// Convert uint64 to int64 safely and add to volume
			volume += int64(singleTransfer.Value)
		} else if singleTransfer.BlockTimestamp <= targetBlockTimestamp {
			break
		} else {
			continue
		}
	}

	return volume
}
