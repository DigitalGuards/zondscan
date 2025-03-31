package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreContract stores or merges contract information in the database
func StoreContract(contract models.ContractInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	collection := configs.GetContractsCollection()
	filter := bson.M{"address": contract.Address}

	// Attempt to find existing contract document
	var existingContract models.ContractInfo
	err := collection.FindOne(ctx, filter).Decode(&existingContract)

	updateData := contract

	if err == nil {
		// Existing contract found, merge new data into it
		configs.Logger.Debug("Found existing contract, merging data", zap.String("address", contract.Address))
		merged := existingContract

		// Merge fields from the new 'contract' object, only if the new value is non-empty/non-zero
		// and the existing value *is* empty/zero. This prioritizes data from the creation tx.
		if merged.CreatorAddress == "" && contract.CreatorAddress != "" {
			merged.CreatorAddress = contract.CreatorAddress
		}
		if merged.CreationTransaction == "" && contract.CreationTransaction != "" {
			merged.CreationTransaction = contract.CreationTransaction
		}
		if merged.CreationBlockNumber == "" && contract.CreationBlockNumber != "" {
			merged.CreationBlockNumber = contract.CreationBlockNumber
		}
		if merged.ContractCode == "" && contract.ContractCode != "" && contract.ContractCode != "0x" {
			merged.ContractCode = contract.ContractCode
		}
		if merged.Status == "" && contract.Status != "" {
			merged.Status = contract.Status
		} else if contract.Status != "" && merged.Status != contract.Status {
			merged.Status = contract.Status
		}

		// For token info, update if the new info seems more complete or explicitly provided
		merged.IsToken = contract.IsToken
		if contract.IsToken {
			if merged.Name == "" && contract.Name != "" {
				merged.Name = contract.Name
			}
			if merged.Symbol == "" && contract.Symbol != "" {
				merged.Symbol = contract.Symbol
			}
			if merged.Decimals == 0 && contract.Decimals != 0 {
				merged.Decimals = contract.Decimals
			}
			if merged.TotalSupply == "" && contract.TotalSupply != "" {
				merged.TotalSupply = contract.TotalSupply
			}
		} else {
			// If it's not a token according to new info, clear token fields
			merged.Name = ""
			merged.Symbol = ""
			merged.Decimals = 0
			merged.TotalSupply = ""
		}

		// Always update the timestamp
		merged.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		updateData = merged

	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		configs.Logger.Error("Failed to check for existing contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	opts := options.Update().SetUpsert(true)
	update := bson.M{"$set": updateData}

	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store/merge contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully stored/merged contract", zap.String("address", updateData.Address))
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
				CreationBlockNumber: tx.BlockNumber,
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

		// Check if the destination address is a contract
		isContract = IsAddressContract(to)
	}

	return to, contractAddress, statusTx, isContract
}

// IsAddressContract checks if an address is a contract by querying the contractCode collection
// and falling back to RPC getCode call if not found
func IsAddressContract(address string) bool {
	// First check our database
	contract := getContractFromDB(address)
	if contract != nil {
		return true
	}

	// If not in database, check via RPC
	code, err := rpc.GetCode(address, "latest")
	if err != nil {
		configs.Logger.Error("Failed to get code for address",
			zap.String("address", address),
			zap.Error(err))
		return false
	}

	// If code is not empty/0x, it's a contract
	isContract := code != "" && code != "0x" && code != "0x0"

	// If it's a contract, store it in our database
	if isContract {
		configs.Logger.Info("Detected existing contract",
			zap.String("address", address))

		// Get token information
		name, symbol, decimals, isToken := rpc.GetTokenInfo(address)

		// Get total supply if it's a token
		var totalSupply string
		if isToken {
			totalSupply, err = rpc.GetTokenTotalSupply(address)
			if err != nil {
				configs.Logger.Error("Failed to get token total supply",
					zap.String("address", address),
					zap.Error(err))
			}
		}

		// Store the contract
		contract := models.ContractInfo{
			Address:      address,
			Status:       "0x1", // Assume successful
			IsToken:      isToken,
			Name:         name,
			Symbol:       symbol,
			Decimals:     decimals,
			TotalSupply:  totalSupply,
			ContractCode: code,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		}

		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to store detected contract",
				zap.String("address", address),
				zap.Error(err))
		}
	}

	return isContract
}

// getContractFromDB retrieves contract information from the contractCode collection
// Local version to avoid naming conflicts
func getContractFromDB(address string) *models.ContractInfo {
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo
	err := collection.FindOne(context.Background(), bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
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
			// Get total supply for token with missing supply
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
