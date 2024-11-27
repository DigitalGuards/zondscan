package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts() ([]models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.Transfer
	defer cancel()

	filter := primitive.D{{Key: "contractAddress", Value: primitive.D{{Key: "$exists", Value: true}}}}
	results, err := configs.TransferCollections.Find(ctx, filter, options.Find())

	if err != nil {
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransfer models.Transfer
		if err = results.Decode(&singleTransfer); err != nil {
			fmt.Println(err)
		}
		transactions = append(transactions, singleTransfer)
	}

	return transactions, nil
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

	filter := primitive.D{{Key: "contractAddress", Value: address}}
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
