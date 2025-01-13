package db

import (
    "Zond2mongoDB/configs"
    "Zond2mongoDB/models"
    "Zond2mongoDB/rpc"
    "context"
    "encoding/hex"
    "strconv"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.uber.org/zap"
)

func processContracts(tx *models.Transaction) ([]byte, []byte, uint8, bool) {
    var to []byte
    var contractAddressByte []byte
    var statusTx uint64
    isContract := false

    from, err := hex.DecodeString(tx.From[2:])
    if err != nil {
        configs.Logger.Warn("Failed to hex decode string", zap.Error(err))
    }

    if tx.To != "" {
        to, err = hex.DecodeString(tx.To[2:])
        if err != nil {
            configs.Logger.Warn("Failed to hex decode string", zap.Error(err))
        }
        contractAddressByte = nil
    } else {
        if tx.Type != "0x3" {
            to = nil

            contractAddress, status, err := rpc.GetContractAddress(tx.Hash)
            if err != nil {
                configs.Logger.Warn("Failed to do rpc request", zap.Error(err))
            }

            contractAddressByte, err = hex.DecodeString(contractAddress[2:])
            if err != nil {
                configs.Logger.Warn("Failed to hex decode string", zap.Error(err))
            }

            statusTx, err = strconv.ParseUint(status, 0, 8)
            if err != nil {
                configs.Logger.Warn("Failed to hex decode string", zap.Error(err))
            }

            if statusTx == 1 {
                isContract = true

                // Get contract code
                code, err := rpc.GetCode(contractAddress, "latest")
                if err != nil {
                    configs.Logger.Warn("Failed to get contract code", zap.Error(err))
                } else {
                    // Convert hex code to bytes
                    codeByte, err := hex.DecodeString(code[2:]) // Remove "0x" prefix
                    if err != nil {
                        configs.Logger.Warn("Failed to decode contract code", zap.Error(err))
                        return from, to, uint8(statusTx), isContract
                    }

                    // Try to get token info
                    name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)
                    
                    contractInfo := &models.ContractInfo{
                        ContractCreatorAddress: from,
                        ContractAddress:        contractAddressByte,
                        ContractCode:           codeByte,
                        TokenName:              name,    // Will be empty for non-tokens
                        TokenSymbol:            symbol,  // Will be empty for non-tokens
                        TokenDecimals:          decimals, // Will be 0 for non-tokens
                        IsToken:                isToken,
                    }

                    configs.Logger.Info("Processing contract",
                        zap.String("address", contractAddress))

                    // Use upsert to update existing contract or insert new one
                    filter := bson.M{"contractAddress": contractAddressByte}
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

    return from, to, uint8(statusTx), isContract
}

// ContractCodeCollection inserts a new contract into the database
func ContractCodeCollection(contractCreatorAddress []byte, contractAddress []byte, code []byte) (*mongo.InsertOneResult, error) {
    var contractInfo models.ContractInfo
    contractInfo.ContractCreatorAddress = contractCreatorAddress
    contractInfo.ContractAddress = contractAddress
    contractInfo.ContractCode = code

    // Try to get token information if we have a valid contract address
    if len(contractAddress) > 0 {
        addrHex := "0x" + hex.EncodeToString(contractAddress)
        name, symbol, decimals, isToken := rpc.GetTokenInfo(addrHex)
        contractInfo.TokenName = name
        contractInfo.TokenSymbol = symbol
        contractInfo.TokenDecimals = decimals
        contractInfo.IsToken = isToken
    }

    return configs.ContractCodeCollection.InsertOne(context.Background(), contractInfo)
}
