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

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Set up schema validation for dailyTransactionsVolume collection
	db := client.Database("qrldata")
	validator := bson.M{
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

	cmd := bson.D{
		{"collMod", "dailyTransactionsVolume"},
		{"validator", validator},
		{"validationLevel", "strict"},
	}

	err = db.RunCommand(context.Background(), cmd).Err()
	if err != nil {
		// If collection doesn't exist, create it with the validator
		if err.Error() == "not found" {
			opts := options.CreateCollection().SetValidator(validator)
			err = db.CreateCollection(context.Background(), "dailyTransactionsVolume", opts)
			if err != nil {
				log.Printf("Warning: Could not create collection with validator: %v", err)
			}
		} else {
			log.Printf("Warning: Could not set up validator: %v", err)
		}
	}

	return client
}

// Client instance
var DB *mongo.Client = ConnectDB()

// getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	collection := client.Database("qrldata").Collection(collectionName)
	return collection
}
