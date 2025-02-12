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
func StoreContract(contract models.ContractInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"address": contract.Address}
	update := bson.M{"$set": contract}

	_, err := configs.GetContractsCollection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store contract",
			zap.String("address", contract.Address),
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

			// Get contract code
			contractCode, err := rpc.GetCode(contractAddress, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contractAddress),
					zap.Error(err))
			}

			// Get token information
			name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)

			// Get total supply if it's a token
			var totalSupply string
			if isToken {
				totalSupply, err = rpc.GetTokenTotalSupply(contractAddress)
				if err != nil {
					configs.Logger.Error("Failed to get token total supply",
						zap.String("address", contractAddress),
						zap.Error(err))
				}
			}

			// Store complete contract information
			contract := models.ContractInfo{
				Address:             contractAddress,
				Status:              statusTx,
				IsToken:             isToken,
				Name:                name,
				Symbol:              symbol,
				Decimals:            decimals,
				TotalSupply:         totalSupply,
				ContractCode:        contractCode,
				CreatorAddress:      tx.From,
				CreationTransaction: tx.Hash,
				UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
			}

			// Store the contract
			err = StoreContract(contract)
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

// ReprocessIncompleteContracts finds and updates contracts with missing information
func ReprocessIncompleteContracts() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find contracts with missing information
	filter := bson.M{
		"$or": []bson.M{
			{"contractCode": ""},
			{"isToken": true, "totalSupply": ""},
			{"isToken": false, "name": "", "symbol": ""},
		},
	}

	cursor, err := configs.GetContractsCollection().Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to query incomplete contracts", zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	var processedCount int
	for cursor.Next(ctx) {
		var contract models.ContractInfo
		if err := cursor.Decode(&contract); err != nil {
			configs.Logger.Error("Failed to decode contract", zap.Error(err))
			continue
		}

		// Get contract code if missing
		if contract.ContractCode == "" {
			contractCode, err := rpc.GetCode(contract.Address, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contract.Address),
					zap.Error(err))
			} else {
				contract.ContractCode = contractCode
			}
		}

		// Get token information if missing
		if !contract.IsToken && contract.Name == "" && contract.Symbol == "" {
			name, symbol, decimals, isToken := rpc.GetTokenInfo(contract.Address)
			if isToken {
				contract.IsToken = isToken
				contract.Name = name
				contract.Symbol = symbol
				contract.Decimals = decimals

				// Get total supply for new tokens
				totalSupply, err := rpc.GetTokenTotalSupply(contract.Address)
				if err != nil {
					configs.Logger.Error("Failed to get token total supply",
						zap.String("address", contract.Address),
						zap.Error(err))
				} else {
					contract.TotalSupply = totalSupply
				}
			}
		} else if contract.IsToken && contract.TotalSupply == "" {
			// Get total supply for existing tokens that are missing it
			totalSupply, err := rpc.GetTokenTotalSupply(contract.Address)
			if err != nil {
				configs.Logger.Error("Failed to get token total supply",
					zap.String("address", contract.Address),
					zap.Error(err))
			} else {
				contract.TotalSupply = totalSupply
			}
		}

		contract.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		// Update the contract
		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to update contract",
				zap.String("address", contract.Address),
				zap.Error(err))
			continue
		}

		processedCount++
		if processedCount%100 == 0 {
			configs.Logger.Info("Reprocessing progress",
				zap.Int("processed_contracts", processedCount))
		}
	}

	if err := cursor.Err(); err != nil {
		configs.Logger.Error("Cursor error while reprocessing contracts", zap.Error(err))
		return err
	}

	configs.Logger.Info("Completed reprocessing incomplete contracts",
		zap.Int("total_processed", processedCount))
	return nil
}

// StartContractReprocessingJob starts a background job to periodically reprocess incomplete contracts
func StartContractReprocessingJob() {
	go func() {
		for {
			configs.Logger.Info("Starting contract reprocessing job")

			err := ReprocessIncompleteContracts()
			if err != nil {
				configs.Logger.Error("Contract reprocessing job failed", zap.Error(err))
			}

			// Wait for 1 hour before next run
			time.Sleep(1 * time.Hour)
		}
	}()
}
