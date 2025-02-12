package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenBalance stores or updates a token balance in the database
func StoreTokenBalance(balance models.TokenBalance) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"contractAddress": balance.ContractAddress,
		"holderAddress":  balance.HolderAddress,
	}
	update := bson.M{"$set": balance}

	_, err := configs.GetTokenBalancesCollection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store token balance",
			zap.String("contract", balance.ContractAddress),
			zap.String("holder", balance.HolderAddress),
			zap.Error(err))
		return err
	}

	return nil
}

// GetTokenBalance retrieves a token balance from the database
func GetTokenBalance(contractAddress, holderAddress string) (*models.TokenBalance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var balance models.TokenBalance
	err := configs.GetTokenBalancesCollection().FindOne(ctx, bson.M{
		"contractAddress": contractAddress,
		"holderAddress":  holderAddress,
	}).Decode(&balance)

	if err != nil {
		return nil, err
	}

	return &balance, nil
}

// UpdateTokenBalance updates or creates a token balance record
func UpdateTokenBalance(contractAddress, holderAddress, blockNumber string) error {
	// Get current balance from RPC
	balance, err := rpc.GetTokenBalance(contractAddress, holderAddress)
	if err != nil {
		return err
	}

	// Create or update balance record
	tokenBalance := models.TokenBalance{
		ID:              primitive.NewObjectID(),
		ContractAddress: contractAddress,
		HolderAddress:  holderAddress,
		Balance:        balance,
		BlockNumber:    blockNumber,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	return StoreTokenBalance(tokenBalance)
}

// GetTokenHolders retrieves all holders of a specific token
func GetTokenHolders(contractAddress string) ([]models.TokenBalance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"contractAddress": contractAddress}
	cursor, err := configs.GetTokenBalancesCollection().Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var balances []models.TokenBalance
	if err = cursor.All(ctx, &balances); err != nil {
		return nil, err
	}

	return balances, nil
}
