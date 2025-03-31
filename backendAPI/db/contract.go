package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string) ([]models.ContractInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the main model directly, it now has correct BSON tags
	var contracts []models.ContractInfo

	// Base filter
	filter := bson.D{}

	// Add search if provided, using correct field names
	if search != "" {
		// Zond addresses start with 'Z'. Search assumes the provided string is the correct format.
		filter = bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "address", Value: search}},        // Use correct field name
				bson.D{{Key: "creatorAddress", Value: search}}, // Use correct field name
				// Add other searchable fields if needed (e.g., name, symbol for tokens)
				// bson.D{{Key: "name", Value: primitive.Regex{Pattern: search, Options: "i"}}}, // Case-insensitive search for name
				// bson.D{{Key: "symbol", Value: primitive.Regex{Pattern: search, Options: "i"}}}, // Case-insensitive search for symbol
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

	// Decode directly into the slice of models.ContractInfo
	if err := cursor.All(ctx, &contracts); err != nil {
		return nil, 0, err
	}

	// No need for manual mapping anymore

	// Return empty slice instead of nil if no contracts found
	if contracts == nil {
		contracts = make([]models.ContractInfo, 0)
	}

	return contracts, total, nil
}

func ReturnContractCode(query string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// No need to manipulate the query string, assume it's the correct Zond format (starts with 'Z')
	// The query could be either a contract address or a creator address.

	// First, try searching by 'address' field
	filter := bson.M{"address": query}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// If not found by address, try searching by 'creatorAddress'
			fmt.Printf("No contract found by address %s, trying creatorAddress...\n", query)
			filter = bson.M{"creatorAddress": query}
			err = configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)
			if err == mongo.ErrNoDocuments {
				// Not found by either field
				fmt.Printf("No contract found by creatorAddress either for %s\n", query)
				return result, err // Return original ErrNoDocuments from the second attempt
			} else if err != nil {
				// Error during the second search attempt
				fmt.Printf("Error querying contract by creatorAddress %s: %v\n", query, err)
				return result, err
			}
			// Found by creatorAddress on the second attempt
		} else {
			// Error during the first search attempt (by address)
			fmt.Printf("Error querying contract by address %s: %v\n", query, err)
			return result, err
		}
		// If we reach here, it means we found the contract (either by address or creatorAddress)
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
