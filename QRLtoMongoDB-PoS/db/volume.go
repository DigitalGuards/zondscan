package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	update := bson.M{
		"$set": bson.M{
			"volume": volume,
		},
	}

	updateResult, err := configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, bson.M{}, update, options.Update().SetUpsert(true))
	if err != nil {
		configs.Logger.Warn("Failed to update daily transaction volume: ", zap.Error(err))
	} else {
		configs.Logger.Info("Update Result: ", zap.Any("UpdateResult", updateResult))
	}
}

func GetVolumeFromBlockTimestamps(targetBlockTimestamp uint64, currentBlockTimestamp uint64, latestBlockNumber uint64) float32 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var volume float32
	defer cancel()

	options := options.Find().SetProjection(bson.M{"blockNumber": 1, "blockTimestamp": 1, "value": 1}).SetSort(bson.D{{"blockNumber", -1}})

	results, err := configs.TransferCollections.Find(ctx, bson.D{}, options)

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
			volume += float32(singleTransfer.Value) / configs.QUANTA
		} else if singleTransfer.BlockTimestamp <= targetBlockTimestamp {
			break
		} else {
			continue
		}
	}

	return volume
}
