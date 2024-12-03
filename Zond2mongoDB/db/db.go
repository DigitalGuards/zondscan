package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func GetLatestBlockHashHeaderFromDB(number uint64) string {
	if number > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		options := options.FindOne().SetProjection(bson.M{"result.hash": 1})

		var singleHashHeader models.ZondDatabaseBlock

		err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.number": number}, options).Decode(&singleHashHeader)

		if err != nil {
			configs.Logger.Warn("Failed to FindOne in the blocks collection: ", zap.Error(err))
		}

		return string(singleHashHeader.Result.Hash)
	} else {
		var Zond models.Zond
		var ZondNew models.ZondDatabaseBlock

		blockresult := rpc.GetBlockByNumberMainnet(0)

		json.Unmarshal([]byte(blockresult), &Zond)
		ZondNew = ConvertModelsUint64(Zond)
		return ZondNew.Result.Hash
	}
}

func GetBlockNumberFromHash(parenthash string) uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	options := options.FindOne().SetProjection(bson.M{"result.number": 1})

	var singleBlockNumber models.ZondDatabaseBlock

	err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.hash": parenthash}, options).Decode(&singleBlockNumber)

	if err != nil {
		configs.Logger.Warn("Failed to do FindOne in the blocks collection: ", zap.Error(err))
	}

	return singleBlockNumber.Result.Number
}

func GetLatestBlockNumber() models.Result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{}
	options := options.FindOne().SetProjection(bson.M{"result.number": 1, "result.timestamp": 1}).SetSort(bson.M{"result.number": -1})

	var Zond models.ZondDatabaseBlock

	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&Zond)

	if err != nil {
		configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
	}

	return Zond.Result
}

func GetLatestBlockNumberFromDB() uint64 {
	if IsCollectionsExist() == false {
		return 0
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		filter := bson.D{}
		options := options.FindOne().SetProjection(bson.M{"result.number": 1}).SetSort(bson.M{"result.number": -1})

		var Zond models.ZondDatabaseBlock

		err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&Zond)

		if err != nil {
			configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
		}

		return Zond.Result.Number
	}
}

func IsCollectionsExist() bool {
	cNames := configs.GetListCollectionNames(configs.DB)

	collectionsExist := true
	if len(cNames) == 0 {
		collectionsExist = false
	}

	return collectionsExist
}

func CountWallets() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := configs.AddressesCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		configs.Logger.Info("Failed to find wallet count document", zap.Error(err))
		return 0
	}

	err = updateWalletCountInDB(results)
	if err != nil {
		configs.Logger.Info("Error updating wallet count in DB:", zap.Error(err))
	}

	return results
}

func updateWalletCountInDB(count int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}

	update := bson.M{
		"$set": bson.M{
			"count": count,
		},
	}

	_, err := configs.WalletCountCollections.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))

	return err
}

type TransactionsVolume struct {
	Volume int64 `bson:"volume"`
}

func GetDailyTransactionVolume() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endTimeStamp := time.Now().Unix()
	startTimeStamp := endTimeStamp - (24 * 60 * 60)

	configs.Logger.Info("Start timestamp: ", zap.Int64("startTimeStamp", startTimeStamp))
	configs.Logger.Info("End timestamp: ", zap.Int64("endTimeStamp", endTimeStamp))

	filter := bson.M{
		"txType": 2,
		"timeStamp": bson.M{
			"$gte": startTimeStamp,
			"$lt":  endTimeStamp,
		},
	}

	cursor, err := configs.TransactionByAddressCollections.Find(ctx, filter)
	if err != nil {
		configs.Logger.Warn("Failed to find transactions in the given time range: ", zap.Error(err))
		return 0
	}
	defer cursor.Close(ctx)

	var totalVolume int64
	for cursor.Next(ctx) {
		var transaction models.TransactionByAddress
		if err = cursor.Decode(&transaction); err != nil {
			configs.Logger.Warn("Failed to decode transaction: ", zap.Error(err))
			continue
		}
		// Convert float32 amount to int64, values are already in smallest unit
		totalVolume += int64(transaction.Amount)
		configs.Logger.Info("Transaction Amount: ", zap.Float32("Amount", transaction.Amount))
	}

	configs.Logger.Info("Total Transaction Volume: ", zap.Int64("TotalVolume", totalVolume))

	volumeDoc := TransactionsVolume{
		Volume: totalVolume,
	}

	update := bson.M{
		"$set": volumeDoc,
	}

	updateResult, err := configs.DailyTransactionsVolumeCollections.UpdateOne(ctx, bson.M{}, update, options.Update().SetUpsert(true))
	if err != nil {
		configs.Logger.Warn("Failed to update daily transaction volume: ", zap.Error(err))
	} else {
		configs.Logger.Info("Update Result: ", zap.Any("UpdateResult", updateResult))
	}

	return totalVolume
}
