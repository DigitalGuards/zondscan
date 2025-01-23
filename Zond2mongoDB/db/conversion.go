package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"

	"go.uber.org/zap"
)

// UpdateTransactionStatuses updates the status of transactions in a block
func UpdateTransactionStatuses(block *models.ZondDatabaseBlock) {
	for index := range block.Result.Transactions {
		_, status, err := rpc.GetContractAddress(block.Result.Transactions[index].Hash)
		if err != nil {
			configs.Logger.Warn("Failed to get contract address",
				zap.String("hash", block.Result.Transactions[index].Hash),
				zap.Error(err))
		}
		block.Result.Transactions[index].Status = status
	}
}
