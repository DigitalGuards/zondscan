package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StoreTokenBalance updates the token balance for a given address
func StoreTokenBalance(contractAddress string, holderAddress string, amount string, blockNumber string) error {
	collection := configs.GetTokenBalancesCollection()
	
	// Get current balance from RPC
	balance, err := rpc.GetTokenBalance(contractAddress, holderAddress)
	if err != nil {
		return fmt.Errorf("failed to get token balance: %v", err)
	}

	// Create update document
	update := bson.M{
		"$set": bson.M{
			"contractAddress": contractAddress,
			"holderAddress":  holderAddress,
			"balance":        balance,
			"blockNumber":    blockNumber,
			"updatedAt":      time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Update options
	opts := options.Update().SetUpsert(true)

	// Filter to find existing document
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":  holderAddress,
	}

	// Perform upsert
	_, err = collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update token balance: %v", err)
	}

	return nil
}

// GetTokenBalance retrieves the current token balance for a holder
func GetTokenBalance(contractAddress string, holderAddress string) (*models.TokenBalance, error) {
	collection := configs.GetTokenBalancesCollection()
	var balance models.TokenBalance

	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":  holderAddress,
	}

	err := collection.FindOne(context.Background(), filter).Decode(&balance)
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

// GetTokenHolders retrieves all holders of a specific token
func GetTokenHolders(contractAddress string) ([]models.TokenBalance, error) {
	collection := configs.GetTokenBalancesCollection()
	var balances []models.TokenBalance

	filter := bson.M{"contractAddress": contractAddress}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &balances)
	if err != nil {
		return nil, err
	}

	return balances, nil
}
