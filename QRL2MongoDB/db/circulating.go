package db

import (
	"QRL2MongoDB/configs"
	"context"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Balance is decoded as float64 because the addresses collection stores the
// balance (in QRL units) as a BSON double. Decoding it into an int64 field made
// the driver error on every document, which left the circulating total stuck
// at 0.
type Address struct {
	Balance float64            `bson:"balance"`
	ID      primitive.ObjectID `bson:"_id"`
}

func UpdateTotalBalance() {
	// Use the shared DB connection and collection references instead of opening
	// a separate connection to localhost:27017.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	destCollection := configs.GetCollection(configs.DB, "totalCirculatingSupply")

	// Accumulate in a big.Float so summing many balances does not lose
	// precision, then emit an integer-QRL string to preserve the existing
	// `circulating` string contract the backend reads.
	total := new(big.Float).SetPrec(256)
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

		total.Add(total, new(big.Float).SetFloat64(address.Balance))
	}

	if err := cursor.Err(); err != nil {
		configs.Logger.Error("Cursor iteration error", zap.Error(err))
		return
	}

	// Integer-QRL decimal string (no scientific notation), matching the prior
	// big.Int string contract the backend's `circulating` field expects.
	totalStr := total.Text('f', 0)

	// Upsert the total balance
	filter := primitive.D{{Key: "_id", Value: "totalBalance"}}
	update := primitive.D{
		{Key: "$set", Value: primitive.D{
			{Key: "circulating", Value: totalStr},
		}},
	}

	opts := options.Update().SetUpsert(true)
	_, err = destCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update total balance", zap.Error(err))
		return
	}

	configs.Logger.Info("Successfully updated total circulating supply",
		zap.String("total", totalStr))
}
