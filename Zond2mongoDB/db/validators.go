package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func InsertValidators(validators models.ResultValidator) {
	// Use a more appropriate filter - just use a constant ID for the validator document
	filter := primitive.D{{Key: "_id", Value: "validators"}}
	update := primitive.D{
		{Key: "$set", Value: primitive.D{
			{Key: "jsonrpc", Value: 2},
			{Key: "resultvalidator", Value: validators},
		}},
	}
	opts := options.Update().SetUpsert(true)
	result, err := configs.ValidatorsCollections.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to insert in the validators collection", zap.Error(err))
		return
	}

	if result.UpsertedCount > 0 {
		configs.Logger.Info("Inserted new validator document")
	} else if result.ModifiedCount > 0 {
		configs.Logger.Info("Updated existing validator document")
	}
}

func UpdateValidators(sum uint64, previousHash string) {
	currentEpoch := int(sum) / 128
	parentEpoch := int(GetBlockNumberFromHash(previousHash)) / 128

	// Always update validators on first run or epoch change
	if currentEpoch != parentEpoch {
		configs.Logger.Info("Fetching validators from beacon chain",
			zap.Int("current_epoch", currentEpoch),
			zap.Int("parent_epoch", parentEpoch))

		validators := rpc.GetValidators()

		// Only insert if we got valid data
		if len(validators.ValidatorsBySlotNumber) > 0 {
			InsertValidators(validators)
			configs.Logger.Info("Successfully updated validators",
				zap.Int("num_slots", len(validators.ValidatorsBySlotNumber)))
		} else {
			configs.Logger.Error("Got empty validator data from beacon chain")
		}
	}
}
