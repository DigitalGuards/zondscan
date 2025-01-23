package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func UpdateValidators(blockNumber string, previousHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.number": blockNumber}
	update := bson.M{"$set": bson.M{"previousHash": previousHash}}

	_, err := configs.BlocksCollections.UpdateOne(ctx, filter, update)
	if err != nil {
		configs.Logger.Info("Failed to update validator document", zap.Error(err))
	}
}

func InsertValidators(validators models.ResultValidator) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clear existing validators
	_, err := configs.ValidatorsCollections.DeleteMany(ctx, bson.M{})
	if err != nil {
		configs.Logger.Info("Failed to delete validator documents", zap.Error(err))
	}

	// Insert new validators
	_, err = configs.ValidatorsCollections.InsertOne(ctx, validators)
	if err != nil {
		configs.Logger.Info("Failed to insert validator document", zap.Error(err))
	}
}

func GetBlockNumberFromHash(hash string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.hash": hash}
	options := options.FindOne().SetProjection(bson.M{"result.number": 1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to get block number from hash", zap.Error(err))
		return "0x0"
	}

	return block.Result.Number
}
