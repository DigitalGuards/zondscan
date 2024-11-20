package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"QRLtoMongoDB-PoS/rpc"
	"context"
	"encoding/hex"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func processContracts(tx *models.Transaction) ([]byte, []byte, uint8, bool) {
	var to []byte
	var contractAddressByte []byte
	var statusTx uint64
	isContract := false

	from, err := hex.DecodeString(tx.From[2:])
	if err != nil {
		configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
	}

	if tx.To != "" {
		to, err = hex.DecodeString(tx.To[2:])
		if err != nil {
			configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
		}
		contractAddressByte = nil
	} else {
		if tx.Type != "0x3" {
			to = nil

			contractAddress, status, err := rpc.GetContractAddress(tx.Hash)
			if err != nil {
				configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
			}

			contractAddressByte, err = hex.DecodeString(contractAddress[2:])
			if err != nil {
				configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
			}

			statusTx, err = strconv.ParseUint(status, 0, 8)
			if err != nil {
				configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
			}

			contractCode, err := rpc.GetCode(contractAddress, tx.BlockNumber)
			if err != nil {
				configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
			}

			contractCodeByte, err := hex.DecodeString(contractCode[2:])
			if err != nil {
				configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
			}

			contractCreatorAddress := from

			contractAddressByte, err = hex.DecodeString(contractAddress[2:])
			if err != nil {
				configs.Logger.Warn("Failed to hex decode string: ", zap.Error(err))
			}

			if len(contractCodeByte) <= 24576 {
				ContractCodeCollection(contractCreatorAddress, contractAddressByte, contractCodeByte)
			}

			if contractCode != "" {
				isContract = true
			}
		}
	}

	return to, contractAddressByte, uint8(statusTx), isContract
}

func ContractCodeCollection(contractCreatorAddress []byte, contractAddress []byte, code []byte) (*mongo.InsertOneResult, error) {
	doc := bson.D{{"contractCreatorAddress", contractCreatorAddress}, {"contractAddress", contractAddress}, {"contractCode", code}}
	result, err := configs.ContractCodeCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the contractcode collection: ", zap.Error(err))
	}

	return result, err
}
