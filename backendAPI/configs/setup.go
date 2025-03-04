package configs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Client instance
var DB *mongo.Client
var dbOnce sync.Once

// ConnectDB establishes a connection to MongoDB
// It uses a sync.Once to ensure the connection is only established once
func ConnectDB() *mongo.Client {
	dbOnce.Do(func() {
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

		// Set the global DB variable
		DB = client
	})

	return DB
}

func createIndexes(db *mongo.Database) {
	ctx := context.Background()

	// Define required indexes
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

	// Check and create indexes if needed
	collections := map[string][]mongo.IndexModel{
		"blocks":       blocksIndexes,
		"transactions": transactionsIndexes,
	}

	for collName, indexes := range collections {
		// First check if collection exists
		exists, err := collectionExists(db, collName)
		if err != nil {
			log.Printf("Warning: Could not check if collection %s exists: %v", collName, err)
			continue
		}

		if !exists {
			log.Printf("Collection %s does not exist, skipping index creation", collName)
			continue
		}

		// Check if indexes already exist
		existingIndexes, err := getExistingIndexes(db, collName)
		if err != nil {
			log.Printf("Warning: Could not retrieve existing indexes for %s: %v", collName, err)
			continue
		}

		// Create only missing indexes
		var missingIndexes []mongo.IndexModel
		for _, idx := range indexes {
			if name := idx.Options.Name; name != nil {
				indexName := *name
				if !indexExists(existingIndexes, indexName) {
					missingIndexes = append(missingIndexes, idx)
				}
			} else {
				missingIndexes = append(missingIndexes, idx)
			}
		}

		if len(missingIndexes) == 0 {
			log.Printf("All required indexes for collection %s exist", collName)
			continue
		}

		// Create only missing indexes
		_, err = db.Collection(collName).Indexes().CreateMany(ctx, missingIndexes)
		if err != nil {
			log.Printf("Warning: Could not create indexes for %s: %v", collName, err)
		} else {
			log.Printf("Created missing indexes for collection %s", collName)
		}
	}
}

// collectionExists checks if a collection exists in the database
func collectionExists(db *mongo.Database, collectionName string) (bool, error) {
	collections, err := db.ListCollectionNames(context.Background(), bson.M{"name": collectionName})
	if err != nil {
		return false, err
	}
	return len(collections) > 0, nil
}

// getExistingIndexes retrieves all existing indexes for a collection
func getExistingIndexes(db *mongo.Database, collectionName string) ([]bson.M, error) {
	cursor, err := db.Collection(collectionName).Indexes().List(context.Background())
	if err != nil {
		return nil, err
	}

	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	return results, nil
}

// indexExists checks if an index with the given name exists in the collection
func indexExists(indexes []bson.M, indexName string) bool {
	for _, idx := range indexes {
		if name, ok := idx["name"].(string); ok && name == indexName {
			return true
		}
	}
	return false
}

func ensureCollection(db *mongo.Database, name string, validator bson.M) {
	// First check if collection exists using the previously defined function
	exists, err := collectionExists(db, name)
	if err != nil {
		log.Printf("Warning: Could not check if collection %s exists: %v", name, err)
		return
	}

	if !exists {
		// Collection doesn't exist, create it with validator
		if validator != nil {
			opts := options.CreateCollection().SetValidator(validator)
			err = db.CreateCollection(context.Background(), name, opts)
		} else {
			err = db.CreateCollection(context.Background(), name, nil)
		}

		if err != nil {
			log.Printf("Warning: Could not create collection %s: %v", name, err)
		} else {
			log.Printf("Created collection %s", name)
		}
		return
	}

	// Collection exists, update validator if one is provided
	if validator != nil {
		cmd := bson.D{
			{Key: "collMod", Value: name},
			{Key: "validator", Value: validator},
			{Key: "validationLevel", Value: "strict"},
		}

		err := db.RunCommand(context.Background(), cmd).Err()
		if err != nil {
			log.Printf("Warning: Could not update validator for %s: %v", name, err)
		} else {
			log.Printf("Updated validator for collection %s", name)
		}
	} else {
		log.Printf("Collection %s exists, no validator provided", name)
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

// Getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	// Ensure DB is initialized
	if client == nil {
		client = ConnectDB()
	}
	collection := client.Database("qrldata-z").Collection(collectionName)
	return collection
}
