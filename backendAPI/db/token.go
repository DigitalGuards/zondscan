package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// GetTokenBalancesByAddress returns all token balances for a given wallet address
// with token metadata (name, symbol, decimals) included
func GetTokenBalancesByAddress(address string) ([]models.TokenBalance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Normalize address to Z-prefix format (QRL standard)
	// DB stores as "Z" + lowercase hex, e.g. "Z2019ea08f4e..."
	normalizedAddress := address

	// Convert 0x prefix to Z prefix if needed
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		normalizedAddress = "Z" + strings.ToLower(address[2:])
	} else if strings.HasPrefix(strings.ToLower(address), "z") {
		// Ensure uppercase Z with lowercase hex
		normalizedAddress = "Z" + strings.ToLower(address[1:])
	} else {
		// No prefix, add Z
		normalizedAddress = "Z" + strings.ToLower(address)
	}

	// Search for the normalized address
	searchAddresses := []string{normalizedAddress}

	collection := configs.GetCollection(configs.DB, "tokenBalances")

	// Aggregation pipeline to join with contractCode for token metadata
	pipeline := []bson.M{
		// Match token balances for this address (case-insensitive)
		{
			"$match": bson.M{
				"holderAddress": bson.M{"$in": searchAddresses},
			},
		},
		// Join with contractCode collection to get token metadata
		{
			"$lookup": bson.M{
				"from":         "contractCode",
				"localField":   "contractAddress",
				"foreignField": "address",
				"as":           "tokenInfo",
			},
		},
		// Unwind the tokenInfo array (should be single element)
		{
			"$unwind": bson.M{
				"path":                       "$tokenInfo",
				"preserveNullAndEmptyArrays": true,
			},
		},
		// Project final structure with token metadata
		{
			"$project": bson.M{
				"contractAddress": 1,
				"holderAddress":   1,
				"balance":         1,
				"blockNumber":     1,
				"updatedAt":       1,
				"name":            "$tokenInfo.name",
				"symbol":          "$tokenInfo.symbol",
				"decimals":        "$tokenInfo.decimals",
			},
		},
		// Convert balance string to decimal for proper numeric sorting
		{
			"$addFields": bson.M{
				"balanceDecimal": bson.M{"$toDecimal": "$balance"},
			},
		},
		// Sort by balance descending (highest value tokens first)
		{
			"$sort": bson.M{"balanceDecimal": -1},
		},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.TokenBalance
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil
	if results == nil {
		results = make([]models.TokenBalance, 0)
	}

	return results, nil
}
