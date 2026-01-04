package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string, isTokenFilter *bool) ([]models.ContractInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the main model directly, it now has correct BSON tags
	var contracts []models.ContractInfo

	// Base filter
	filter := bson.D{}

	// Add isToken filter if specified
	if isTokenFilter != nil {
		filter = append(filter, bson.E{Key: "isToken", Value: *isTokenFilter})
	}

	// Add search if provided, using correct field names
	if search != "" {
		// Normalize the search address to lowercase for case-insensitive lookup
		normalizedSearch := strings.ToLower(search)

		// Also try with Z prefix in case user pastes address without it
		searchWithZ := normalizedSearch
		if !strings.HasPrefix(normalizedSearch, "z") {
			searchWithZ = "z" + normalizedSearch
		}

		// Zond addresses start with 'Z'. Search for both with and without Z prefix.
		searchFilter := bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "address", Value: bson.M{"$in": bson.A{normalizedSearch, searchWithZ}}}},        // Match contract address
				bson.D{{Key: "creatorAddress", Value: bson.M{"$in": bson.A{normalizedSearch, searchWithZ}}}}, // Match creator address
				bson.D{{Key: "name", Value: bson.D{{Key: "$regex", Value: normalizedSearch}, {Key: "$options", Value: "i"}}}}, // Match token name
			}},
		}
		// Combine with existing filter
		if len(filter) > 0 {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, searchFilter}}}
		} else {
			filter = searchFilter
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

	// Normalize address while preserving the Z prefix
	normalizedAddress := address
	if strings.HasPrefix(address, "Z") {
		// Keep the Z prefix and normalize the rest
		normalizedAddress = "Z" + strings.ToLower(address[1:])
	} else {
		// Add Z prefix if missing and normalize
		normalizedAddress = "Z" + strings.ToLower(strings.TrimPrefix(address, "Z"))
	}

	// Query for contract code using the normalized address
	filter := bson.D{{Key: "address", Value: normalizedAddress}}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Log that we couldn't find the contract
			log.Printf("No contract found for address: %s (normalized: %s)", address, normalizedAddress)
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
