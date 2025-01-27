package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string) ([]models.ContractInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var contracts []models.ContractInfo
	defer cancel()

	// Base filter for contracts
	filter := bson.D{}  // All documents in ContractInfo collection are contracts

	// Add search if provided
	if search != "" {
		// Convert search to bytes for comparison with byte fields
		searchBytes, _ := hex.DecodeString(search)
		filter = bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "contractAddress", Value: searchBytes}},
				bson.D{{Key: "contractCreatorAddress", Value: searchBytes}},
			}},
		}
	}

	// Get total count for pagination
	total, err := configs.ContractInfoCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination options
	skip := page * limit
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "_id", Value: -1}})  // Latest first

	cursor, err := configs.ContractInfoCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &contracts); err != nil {
		return nil, 0, err
	}

	return contracts, total, nil
}

func ReturnContractCode(query string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Remove "0x" prefix if present
	query = strings.TrimPrefix(query, "0x")

	// Try to decode the query as a hex address
	address, err := hex.DecodeString(query)
	if err != nil {
		return result, fmt.Errorf("failed to decode hex address: %v", err)
	}

	filter := bson.M{"contractAddress": address}
	err = configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No contract code found for %s\n", query)
		} else {
			fmt.Printf("Error querying contract code %s: %v\n", query, err)
		}
		return result, err
	}

	return result, nil
}

// CountContracts returns the total number of smart contracts
func CountContracts() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.ContractInfoCollection.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, err
	}

	return count, nil
}
