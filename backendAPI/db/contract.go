package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string) ([]models.Transfer, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.Transfer
	defer cancel()

	// Base filter for contracts
	filter := bson.D{
		{Key: "contractAddress", Value: bson.D{{Key: "$exists", Value: true}}},
	}

	// Add search if provided
	if search != "" {
		// Convert search to bytes for comparison with byte fields
		searchBytes, _ := hex.DecodeString(search)
		filter = bson.D{
			{Key: "$and", Value: bson.A{
				filter,
				bson.D{
					{Key: "$or", Value: bson.A{
						bson.D{{Key: "contractAddress", Value: searchBytes}},
						bson.D{{Key: "from", Value: searchBytes}},
						bson.D{{Key: "txHash", Value: searchBytes}},
					}},
				},
			}},
		}
	}

	// Get total count for pagination
	total, err := configs.TransferCollections.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination options
	skip := page * limit
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "blockTimestamp", Value: -1}}) // Sort by timestamp descending

	results, err := configs.TransferCollections.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransfer models.Transfer
		if err = results.Decode(&singleTransfer); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, singleTransfer)
	}

	return transactions, total, nil
}

func ReturnContractCode(query string) (models.ContractCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractCode

	address, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Printf("Error decoding contract address %s: %v\n", query, err)
		return result, err
	}

	filter := bson.M{"contractAddress": address}
	err = configs.ContractCodeCollection.FindOne(ctx, filter).Decode(&result)
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

	// Filter for contracts
	filter := bson.D{
		{Key: "contractAddress", Value: bson.D{{Key: "$exists", Value: true}}},
	}

	count, err := configs.TransferCollections.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}
