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

	// Initialize collections with validators and indexes
	db := client.Database("qrldata-z")

	// Create indexes
	createIndexes(db)

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

	// Initialize collections
	initializeCollections(db)

	return client
}

func createIndexes(db *mongo.Database) {
	ctx := context.Background()

	// Blocks collection indexes
	blocksIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "result.number", Value: -1},
				{Key: "result.timestamp", Value: 1},
			},
			Options: options.Index().SetName("result_number_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "result.hash", Value: 1},
			},
			Options: options.Index().SetName("result_hash"),
		},
	}

	// Transactions collection indexes
	transactionsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "timeStamp", Value: -1},
			},
			Options: options.Index().SetName("timestamp_desc"),
		},
		{
			Keys: bson.D{
				{Key: "txHash", Value: 1},
			},
			Options: options.Index().SetName("tx_hash"),
		},
	}

	// Create indexes
	collections := map[string][]mongo.IndexModel{
		"blocks":       blocksIndexes,
		"transactions": transactionsIndexes,
	}

	for collName, indexes := range collections {
		_, err := db.Collection(collName).Indexes().CreateMany(ctx, indexes)
		if err != nil {
			log.Printf("Warning: Could not create indexes for %s: %v", collName, err)
		} else {
			log.Printf("Created indexes for collection %s", collName)
		}
	}
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

func initializeCollections(db *mongo.Database) {
	ctx := context.Background()

	// Initialize CoinGecko collection with empty document
	_, err := db.Collection("coingecko").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{
			"marketCapUSD": 0.0,
			"priceUSD":     0.0,
			"lastUpdated":  time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Failed to initialize CoinGecko collection: %v", err)
	}

	// Initialize WalletCount collection
	_, err = db.Collection("walletCount").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{"count": int64(0)}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Failed to initialize WalletCount collection: %v", err)
	}

	// Initialize DailyTransactionsVolume collection
	_, err = db.Collection("dailyTransactionsVolume").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{"volume": int64(0)}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Failed to initialize DailyTransactionsVolume collection: %v", err)
	}

	// Initialize TotalCirculatingSupply collection
	_, err = db.Collection("totalCirculatingSupply").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{"circulating": "0"}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Failed to initialize TotalCirculatingSupply collection: %v", err)
	}
}

// Client instance
var DB *mongo.Client = ConnectDB()

// Getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	collection := client.Database("qrldata-z").Collection(collectionName)
	return collection
}
