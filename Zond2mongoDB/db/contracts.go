package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func processContracts(tx *models.Transaction) (string, string, string, bool) {
	var to string
	var contractAddress string
	var statusTx string
	isContract := false

	from := tx.From

	if tx.To != "" {
		to = tx.To
		contractAddress = ""
	} else {
		if tx.Type != "0x3" {
			to = ""

			newContractAddr, status, err := rpc.GetContractAddress(tx.Hash)
			if err != nil {
				configs.Logger.Warn("Failed to do rpc request", zap.Error(err))
			}

			contractAddress = newContractAddr
			statusTx = status
			if status == "0x1" {
				isContract = true

				// Get contract code
				code, err := rpc.GetCode(contractAddress, "latest")
				if err != nil {
					configs.Logger.Warn("Failed to get contract code", zap.Error(err))
				} else {
					// Try to get token info
					name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)

					contractInfo := &models.ContractInfo{
						ContractCreatorAddress: from,
						ContractAddress:        contractAddress,
						ContractCode:           code,
						TokenName:              name,     // Will be empty for non-tokens
						TokenSymbol:            symbol,   // Will be empty for non-tokens
						TokenDecimals:          decimals, // Will be 0 for non-tokens
						IsToken:                isToken,
					}

					configs.Logger.Info("Processing contract",
						zap.String("address", contractAddress))

					// Use upsert to update existing contract or insert new one
					filter := bson.M{"contractAddress": contractAddress}
					update := bson.M{"$set": contractInfo}
					opts := options.Update().SetUpsert(true)

					result, err := configs.ContractCodeCollection.UpdateOne(context.Background(), filter, update, opts)
					if err != nil {
						configs.Logger.Warn("Failed to store contract info", zap.Error(err))
					} else {
						configs.Logger.Info("Upserted contract",
							zap.String("address", contractAddress),
							zap.Int64("modified", result.ModifiedCount))
					}
				}
			}
		}
	}

	return from, to, statusTx, isContract
}

// ContractCodeCollection inserts a new contract into the database
func ContractCodeCollection(contractCreatorAddress string, contractAddress string, code string) (*mongo.InsertOneResult, error) {
	var contractInfo models.ContractInfo
	contractInfo.ContractCreatorAddress = contractCreatorAddress
	contractInfo.ContractAddress = contractAddress
	contractInfo.ContractCode = code

	// Try to get token information if we have a valid contract address
	if contractAddress != "" {
		name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)
		contractInfo.TokenName = name
		contractInfo.TokenSymbol = symbol
		contractInfo.TokenDecimals = decimals
		contractInfo.IsToken = isToken
	}

	return configs.ContractCodeCollection.InsertOne(context.Background(), contractInfo)
}
