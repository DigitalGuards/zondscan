package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string) ([]models.ContractInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a type to handle raw BSON data from MongoDB
	type RawContractInfo struct {
		ContractCreatorAddress string `bson:"contractCreatorAddress"`
		ContractAddress        string `bson:"contractAddress"`
		ContractCode           string `bson:"contractCode"`
		TokenName              string `bson:"tokenName,omitempty"`
		TokenSymbol            string `bson:"tokenSymbol,omitempty"`
		TokenDecimals          uint8  `bson:"tokenDecimals,omitempty"`
		IsToken                bool   `bson:"isToken"`
	}
	var rawContracts []RawContractInfo

	// Base filter for contracts
	filter := bson.D{} // All documents in ContractInfo collection are contracts

	// Add search if provided
	if search != "" {
		// Search is already in hex format with 0x prefix
		filter = bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "contractAddress", Value: search}},
				bson.D{{Key: "contractCreatorAddress", Value: search}},
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
		SetSort(bson.D{{Key: "_id", Value: -1}}) // Latest first

	cursor, err := configs.ContractInfoCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &rawContracts); err != nil {
		return nil, 0, err
	}

	// Convert raw contracts to the response format - direct assignment since formats match
	contracts := make([]models.ContractInfo, len(rawContracts))
	for i, raw := range rawContracts {
		contracts[i] = models.ContractInfo{
			ContractCreatorAddress: raw.ContractCreatorAddress,
			ContractAddress:        raw.ContractAddress,
			ContractCode:           raw.ContractCode,
			TokenName:              raw.TokenName,
			TokenSymbol:            raw.TokenSymbol,
			TokenDecimals:          raw.TokenDecimals,
			IsToken:                raw.IsToken,
		}
	}

	return contracts, total, nil
}

func ReturnContractCode(query string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Ensure address has 0x prefix
	if !strings.HasPrefix(query, "0x") {
		query = "0x" + query
	}

	// Just look up in MongoDB - we already have contracts indexed
	filter := bson.M{"contractAddress": query}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No contract found in DB for %s\n", query)
		} else {
			fmt.Printf("Error querying contract %s: %v\n", query, err)
		}
		return result, err
	}

	// Log successful contract lookup
	if result.IsToken {
		fmt.Printf("Found token contract for %s (Name: %s, Symbol: %s)\n", query, result.TokenName, result.TokenSymbol)
	} else {
		fmt.Printf("Found contract for %s\n", query)
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
