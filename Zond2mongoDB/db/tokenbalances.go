package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/validation"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenBalance updates the token balance for a given address
func StoreTokenBalance(contractAddress string, holderAddress string, amount string, blockNumber string) error {
	// Normalize addresses to canonical Z-prefix form
	contractAddress = validation.ConvertToZAddress(contractAddress)

	// Debug-level log for per-record operations; Info is reserved for batch summaries.
	configs.Logger.Debug("Attempting to store token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.String("transferAmount", amount),
		zap.String("blockNumber", blockNumber))

	// Normalize holder address to canonical Z-prefix form
	holderAddress = validation.ConvertToZAddress(holderAddress)

	// Special handling for zero address (QRL uses Z prefix)
	if holderAddress == "Z0" ||
		holderAddress == configs.QRLZeroAddress ||
		holderAddress == "0x0" ||
		holderAddress == "0x0000000000000000000000000000000000000000" {
		configs.Logger.Debug("Skipping token balance update for zero address",
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
		configs.Logger.Debug("Using default zero balance after RPC failure")
		balance = "0"
	} else {
		configs.Logger.Debug("Retrieved current token balance",
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform upsert
	result, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update token balance in database",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		return fmt.Errorf("failed to update token balance: %v", err)
	}

	configs.Logger.Debug("Token balance update completed",
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, filter).Decode(&balance)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	err = cursor.All(ctx, &balances)
	if err != nil {
		return nil, err
	}

	return balances, nil
}
