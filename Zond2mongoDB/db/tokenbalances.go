package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenBalance updates the token balance for a given address
func StoreTokenBalance(contractAddress string, holderAddress string, amount string, blockNumber string) error {
	configs.Logger.Info("Attempting to store token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.String("transferAmount", amount),
		zap.String("blockNumber", blockNumber))

	// Special handling for zero address (QRL uses Z prefix)
	normalizedHolder := strings.ToLower(holderAddress)
	if normalizedHolder == "z0" ||
		normalizedHolder == strings.ToLower(configs.QRLZeroAddress) ||
		normalizedHolder == "0x0" ||
		normalizedHolder == "0x0000000000000000000000000000000000000000" {
		configs.Logger.Info("Skipping token balance update for zero address",
			zap.String("holderAddress", holderAddress))
		return nil
	}

	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		configs.Logger.Error("Failed to get token balances collection")
		return fmt.Errorf("token balances collection is nil")
	}

	// Get current balance from RPC with more robust error handling
	configs.Logger.Debug("Calling RPC to get current token balance")
	balance, err := rpc.GetTokenBalance(contractAddress, holderAddress)
	if err != nil {
		configs.Logger.Error("Failed to get token balance from RPC",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		// Continue with a zero balance if we can't get the actual balance
		// This allows us to at least record that we tried to update this token balance
		configs.Logger.Info("Using default zero balance after RPC failure")
		balance = "0"
	} else {
		configs.Logger.Info("Retrieved current token balance",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.String("balance", balance))
	}

	// Create update document
	update := bson.M{
		"$set": bson.M{
			"contractAddress": contractAddress,
			"holderAddress":   holderAddress,
			"balance":         balance,
			"blockNumber":     blockNumber,
			"updatedAt":       time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Update options
	opts := options.Update().SetUpsert(true)

	// Filter to find existing document
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
	}

	// Perform upsert
	configs.Logger.Debug("Performing upsert operation for token balance")
	result, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update token balance in database",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		return fmt.Errorf("failed to update token balance: %v", err)
	}

	configs.Logger.Info("Token balance update completed",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.Int64("matchedCount", result.MatchedCount),
		zap.Int64("modifiedCount", result.ModifiedCount),
		zap.Int64("upsertedCount", result.UpsertedCount))

	return nil
}

// GetTokenBalance retrieves the current token balance for a holder
func GetTokenBalance(contractAddress string, holderAddress string) (*models.TokenBalance, error) {
	collection := configs.GetTokenBalancesCollection()
	var balance models.TokenBalance

	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
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
