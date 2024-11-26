package configs

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI(EnvMongoURI()))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	//ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB")

	// Initialize collections with validators
	db := client.Database("qrldata")

	// Daily Transactions Volume
	volumeValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"volume"},
			"properties": bson.M{
				"volume": bson.M{
					"bsonType":    "long",
					"description": "must be a long/int64 and is required",
				},
			},
		},
	}
	ensureCollection(db, "dailyTransactionsVolume", volumeValidator)

	// CoinGecko Data
	coingeckoValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"marketCapUSD", "priceUSD", "lastUpdated"},
			"properties": bson.M{
				"marketCapUSD": bson.M{
					"bsonType":    "double",
					"description": "must be a double and is required",
				},
				"priceUSD": bson.M{
					"bsonType":    "double",
					"description": "must be a double and is required",
				},
				"lastUpdated": bson.M{
					"bsonType":    "date",
					"description": "must be a date and is required",
				},
			},
		},
	}
	ensureCollection(db, "coingecko", coingeckoValidator)

	// Wallet Count
	walletValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"count"},
			"properties": bson.M{
				"count": bson.M{
					"bsonType":    "long",
					"description": "must be a long/int64 and is required",
				},
			},
		},
	}
	ensureCollection(db, "walletCount", walletValidator)

	// Total Circulating Supply
	circulatingValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"circulating"},
			"properties": bson.M{
				"circulating": bson.M{
					"bsonType":    "string",
					"description": "must be a string and is required",
				},
			},
		},
	}
	ensureCollection(db, "totalCirculatingSupply", circulatingValidator)

	return client
}

func ensureCollection(db *mongo.Database, name string, validator bson.M) {
	cmd := bson.D{
		{Key: "collMod", Value: name},
		{Key: "validator", Value: validator},
		{Key: "validationLevel", Value: "strict"},
	}

	err := db.RunCommand(context.Background(), cmd).Err()
	if err != nil {
		// If collection doesn't exist, create it with the validator
		if err.Error() == "not found" {
			opts := options.CreateCollection().SetValidator(validator)
			err = db.CreateCollection(context.Background(), name, opts)
			if err != nil {
				log.Printf("Warning: Could not create collection %s with validator: %v", name, err)
			} else {
				log.Printf("Created collection %s with validator", name)
			}
		} else {
			log.Printf("Warning: Could not set up validator for %s: %v", name, err)
		}
	} else {
		log.Printf("Updated validator for collection %s", name)
	}
}

// Client instance
var DB *mongo.Client = ConnectDB()

// getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	collection := client.Database("qrldata").Collection(collectionName)
	return collection
}
