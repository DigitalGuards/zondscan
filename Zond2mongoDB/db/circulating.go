package db

import (
	"Zond2mongoDB/configs"
	"context"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type Address struct {
	Balance int64              `bson:"balance"`
	ID      primitive.ObjectID `bson:"_id"`
}

func UpdateTotalBalance() {
	// Use the shared DB connection and collection references instead of opening
	// a separate connection to localhost:27017.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	destCollection := configs.GetCollection(configs.DB, "totalCirculatingSupply")

	// Get initial total balance
	total := big.NewInt(0)
	cursor, err := configs.AddressesCollections.Find(ctx, primitive.D{})
	if err != nil {
		configs.Logger.Error("Failed to query addresses", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var address Address
		if err := cursor.Decode(&address); err != nil {
			configs.Logger.Error("Failed to decode address", zap.Error(err))
			continue // Skip this address but continue processing others
		}

		balanceBigInt := new(big.Int)
		balanceBigInt.SetInt64(address.Balance)
		total.Add(total, balanceBigInt)
	}

	if err := cursor.Err(); err != nil {
		configs.Logger.Error("Cursor iteration error", zap.Error(err))
		return
	}

	// Upsert the total balance
	filter := primitive.D{{Key: "_id", Value: "totalBalance"}}
	update := primitive.D{
		{Key: "$set", Value: primitive.D{
			{Key: "circulating", Value: total.String()},
		}},
	}

	opts := options.Update().SetUpsert(true)
	_, err = destCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update total balance", zap.Error(err))
		return
	}

	configs.Logger.Info("Successfully updated total circulating supply",
		zap.String("total", total.String()))
}
