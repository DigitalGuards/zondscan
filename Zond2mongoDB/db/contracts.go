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
	"go.uber.org/zap"
)

// StoreContract stores contract information in the database
func StoreContract(contractAddress string, status string, isToken bool, name string, symbol string, decimals uint8) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contract := models.ContractInfo{
		Address:  contractAddress,
		Status:   status,
		IsToken:  isToken,
		Name:     name,
		Symbol:   symbol,
		Decimals: decimals,
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"address": contractAddress}
	update := bson.M{"$set": contract}

	_, err := configs.GetContractsCollection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store contract",
			zap.String("address", contractAddress),
			zap.Error(err))
		return err
	}

	return nil
}

// GetContract retrieves contract information from the database
func GetContract(address string) (*models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var contract models.ContractInfo
	err := configs.GetContractsCollection().FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %v", err)
	}

	return &contract, nil
}

// UpdateContractStatus updates the status of a contract
func UpdateContractStatus(address string, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"status": status}}
	_, err := configs.GetContractsCollection().UpdateOne(ctx, bson.M{"address": address}, update)
	if err != nil {
		return fmt.Errorf("failed to update contract status: %v", err)
	}

	return nil
}

// processContracts processes contract-related information from a transaction
func processContracts(tx *models.Transaction) (string, string, string, bool) {
	var to string
	var contractAddress string
	var statusTx string
	var isContract bool

	// Check if it's a contract creation transaction
	if tx.To == "" {
		// Get contract address and status from transaction receipt
		var err error
		contractAddress, statusTx, err = rpc.GetContractAddress(tx.Hash)
		if err != nil {
			configs.Logger.Error("Failed to get contract address",
				zap.String("hash", tx.Hash),
				zap.Error(err))
			return "", "", "", false
		}

		if contractAddress != "" {
			isContract = true
			// Store the contract with initial status
			err = StoreContract(contractAddress, statusTx, false, "", "", 0)
			if err != nil {
				configs.Logger.Error("Failed to store contract",
					zap.String("address", contractAddress),
					zap.Error(err))
			}
		}
	} else {
		to = tx.To
		statusTx = tx.Status
		isContract = false
	}

	return to, contractAddress, statusTx, isContract
}
