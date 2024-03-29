package db

import (
	"QRLtoMongoDB-PoS/configs"
	"context"
	"log"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Address struct {
	Balance int64              `bson:"balance"`
	_id     primitive.ObjectID `bson:"_id"`
}

func UpdateTotalBalance() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	destCollection := client.Database("qrldata").Collection("totalCirculatingSupply")

	// Get initial total balance
	total := big.NewInt(0)
	cursor, err := configs.AddressesCollections.Find(context.Background(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var address Address
		err := cursor.Decode(&address)
		if err != nil {
			log.Fatal(err)
		}

		balanceBigInt := new(big.Int)
		balanceBigInt.SetInt64(address.Balance)
		total.Add(total, balanceBigInt)
	}

	// Upsert the total balance to the X collection
	upsert := true
	filter := bson.M{"_id": "totalBalance"}
	update := bson.M{
		"$set": bson.M{
			"circulating": total.String(), // total is now of type *big.Int
		},
	}

	_, err = destCollection.UpdateOne(context.Background(), filter, update, &options.UpdateOptions{
		Upsert: &upsert,
	})
	if err != nil {
		log.Fatal(err)
	}
}
