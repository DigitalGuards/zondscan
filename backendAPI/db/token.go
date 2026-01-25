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
		// Add lowercase version of contractAddress for case-insensitive lookup
		{
			"$addFields": bson.M{
				"contractAddressLower": bson.M{"$toLower": "$contractAddress"},
			},
		},
		// Join with contractCode collection using lowercase addresses
		{
			"$lookup": bson.M{
				"from": "contractCode",
				"let":  bson.M{"contractAddr": "$contractAddressLower"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$eq": []interface{}{
									bson.M{"$toLower": "$address"},
									"$$contractAddr",
								},
							},
						},
					},
				},
				"as": "tokenInfo",
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

// normalizeAddress converts an address to QRL Z-prefix format
func normalizeAddress(address string) string {
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		return "Z" + strings.ToLower(address[2:])
	} else if strings.HasPrefix(strings.ToLower(address), "z") {
		return "Z" + strings.ToLower(address[1:])
	}
	return "Z" + strings.ToLower(address)
}

// GetTokenHolders returns all holders of a specific token contract with pagination
func GetTokenHolders(contractAddress string, page, limit int) ([]models.TokenBalance, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	normalizedContract := normalizeAddress(contractAddress)
	collection := configs.GetCollection(configs.DB, "tokenBalances")

	// Count total holders
	totalCount, err := collection.CountDocuments(ctx, bson.M{"contractAddress": normalizedContract})
	if err != nil {
		return nil, 0, err
	}

	// Aggregation pipeline to get holders sorted by balance
	pipeline := []bson.M{
		{
			"$match": bson.M{"contractAddress": normalizedContract},
		},
		// Convert balance to decimal for proper sorting
		{
			"$addFields": bson.M{
				"balanceDecimal": bson.M{"$toDecimal": "$balance"},
			},
		},
		// Sort by balance descending
		{
			"$sort": bson.M{"balanceDecimal": -1},
		},
		// Pagination
		{
			"$skip": int64(page * limit),
		},
		{
			"$limit": int64(limit),
		},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var holders []models.TokenBalance
	if err := cursor.All(ctx, &holders); err != nil {
		return nil, 0, err
	}

	if holders == nil {
		holders = make([]models.TokenBalance, 0)
	}

	return holders, int(totalCount), nil
}

// GetTokenTransfers returns all transfers for a specific token contract with pagination
func GetTokenTransfers(contractAddress string, page, limit int) ([]models.TokenTransfer, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	normalizedContract := normalizeAddress(contractAddress)
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	// Count total transfers
	totalCount, err := collection.CountDocuments(ctx, bson.M{"contractAddress": normalizedContract})
	if err != nil {
		return nil, 0, err
	}

	// Find with pagination, sorted by block number descending (most recent first)
	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(int64(page * limit)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, bson.M{"contractAddress": normalizedContract}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err := cursor.All(ctx, &transfers); err != nil {
		return nil, 0, err
	}

	if transfers == nil {
		transfers = make([]models.TokenTransfer, 0)
	}

	return transfers, totalCount, nil
}

// GetTokenInfo returns summary information about a token
func GetTokenInfo(contractAddress string) (*models.TokenInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	normalizedContract := normalizeAddress(contractAddress)

	// Get contract info
	contractCollection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo
	err := contractCollection.FindOne(ctx, bson.M{"address": normalizedContract}).Decode(&contract)
	if err != nil {
		return nil, err
	}

	// Count holders
	balanceCollection := configs.GetCollection(configs.DB, "tokenBalances")
	holderCount, err := balanceCollection.CountDocuments(ctx, bson.M{"contractAddress": normalizedContract})
	if err != nil {
		holderCount = 0
	}

	// Count transfers
	transferCollection := configs.GetCollection(configs.DB, "tokenTransfers")
	transferCount, err := transferCollection.CountDocuments(ctx, bson.M{"contractAddress": normalizedContract})
	if err != nil {
		transferCount = 0
	}

	return &models.TokenInfo{
		ContractAddress: contract.ContractAddress,
		Name:            contract.TokenName,
		Symbol:          contract.TokenSymbol,
		Decimals:        int(contract.TokenDecimals),
		TotalSupply:     contract.TotalSupply,
		HolderCount:     int(holderCount),
		TransferCount:   transferCount,
		CreatorAddress:  contract.ContractCreatorAddress,
		CreationTxHash:  contract.CreationTransaction,
		CreationBlock:   contract.CreationBlockNumber,
	}, nil
}

// GetTokenTransferByTxHash returns token transfer info for a given transaction hash
// Returns nil if no token transfer is associated with this transaction
func GetTokenTransferByTxHash(txHash string) (*models.TokenTransfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	// Normalize the transaction hash - syncer stores with 0x prefix in lowercase
	normalizedHash := strings.ToLower(txHash)
	if !strings.HasPrefix(normalizedHash, "0x") {
		normalizedHash = "0x" + normalizedHash
	}

	var transfer models.TokenTransfer
	// First try with 0x prefix (standard storage format)
	err := collection.FindOne(ctx, bson.M{"txHash": normalizedHash}).Decode(&transfer)
	if err == nil {
		return &transfer, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Try without 0x prefix in case storage format varies
	hashWithoutPrefix := strings.TrimPrefix(normalizedHash, "0x")
	err = collection.FindOne(ctx, bson.M{"txHash": hashWithoutPrefix}).Decode(&transfer)
	if err == nil {
		return &transfer, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Try with original hash as-is
	err = collection.FindOne(ctx, bson.M{"txHash": txHash}).Decode(&transfer)
	if err == nil {
		return &transfer, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	return nil, nil
}
