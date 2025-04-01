package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"strings"
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
		// Normalize the search address to lowercase for case-insensitive lookup
		normalizedSearch := strings.ToLower(search)

		// Zond addresses start with 'Z'. Search assumes the provided string is the correct format.
		filter = bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "address", Value: normalizedSearch}},        // Match contract address
				bson.D{{Key: "creatorAddress", Value: normalizedSearch}}, // Match creator address
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

	// Return empty slice instead of nil if no contracts found
	if contracts == nil {
		contracts = make([]models.ContractInfo, 0)
	}

	return contracts, total, nil
}

func ReturnContractCode(address string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Normalize address by converting to lowercase
	normalizedAddress := strings.ToLower(address)

	// Query for contract code using the normalized address
	filter := bson.D{{Key: "address", Value: normalizedAddress}}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Return empty contract code with expected structure
			return models.ContractInfo{
				ContractAddress:        "",
				ContractCreatorAddress: "",
				ContractCode:           "",
				CreationTransaction:    "",
				IsToken:                false,
				Status:                 "",
				TokenDecimals:          0,
				TokenName:              "",
				TokenSymbol:            "",
				UpdatedAt:              "",
			}, nil
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
