package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"QRLtoMongoDB-PoS/rpc"
	"math/big"

	"go.uber.org/zap"
)

func ConvertModelsUint64(Zond models.Zond) models.ZondDatabaseBlock {
	BaseFeePerGas := new(big.Int)
	BaseFeePerGas.SetString(Zond.PreResult.BaseFeePerGas[2:], 16)
	BaseFeePerGas.Uint64()

	GasLimit := new(big.Int)
	GasLimit.SetString(Zond.PreResult.GasLimit[2:], 16)
	GasLimit.Uint64()

	GasUsed := new(big.Int)
	GasUsed.SetString(Zond.PreResult.GasUsed[2:], 16)
	GasUsed.Uint64()

	blockNumber := new(big.Int)
	blockNumber.SetString(Zond.PreResult.Number[2:], 16)
	blockNumber.Uint64()

	Timestamp := new(big.Int)
	Timestamp.SetString(Zond.PreResult.Timestamp[2:], 16)
	Timestamp.Uint64()

	Difficulty := new(big.Int)
	Difficulty.SetString(Zond.PreResult.Difficulty[2:], 16)
	Difficulty.Uint64()

	Size := new(big.Int)
	Size.SetString(Zond.PreResult.Size[2:], 16)
	Size.Uint64()

	TotalDifficulty := new(big.Int)
	TotalDifficulty.SetString(Zond.PreResult.TotalDifficulty[2:], 16)
	TotalDifficulty.Uint64()

	var ZondNew models.ZondDatabaseBlock

	ZondNew.Jsonrpc = Zond.Jsonrpc
	ZondNew.ID = Zond.ID
	ZondNew.Result.BaseFeePerGas = BaseFeePerGas.Uint64()
	ZondNew.Result.GasLimit = GasLimit.Uint64()
	ZondNew.Result.GasUsed = GasUsed.Uint64()
	ZondNew.Result.Hash = Zond.PreResult.Hash
	ZondNew.Result.Number = blockNumber.Uint64()
	ZondNew.Result.ParentHash = Zond.PreResult.ParentHash
	ZondNew.Result.ReceiptsRoot = Zond.PreResult.ReceiptsRoot
	ZondNew.Result.StateRoot = Zond.PreResult.StateRoot
	ZondNew.Result.Timestamp = Timestamp.Uint64()
	ZondNew.Result.Transactions = Zond.PreResult.Transactions
	for index := range ZondNew.Result.Transactions {
		_, status, err := rpc.GetContractAddress(Zond.PreResult.Transactions[index].Hash)
		if err != nil {
			configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
		}
		ZondNew.Result.Transactions[index].Status = status
	}
	ZondNew.Result.TransactionsRoot = Zond.PreResult.TransactionsRoot

	ZondNew.Result.Difficulty = Difficulty.Uint64()
	ZondNew.Result.ExtraData = Zond.PreResult.ExtraData
	ZondNew.Result.LogsBloom = Zond.PreResult.LogsBloom
	ZondNew.Result.Miner = Zond.PreResult.Miner
	ZondNew.Result.MixHash = Zond.PreResult.MixHash
	ZondNew.Result.Nonce = Zond.PreResult.Nonce
	ZondNew.Result.Sha3Uncles = Zond.PreResult.Sha3Uncles
	ZondNew.Result.Size = Size.Uint64()
	ZondNew.Result.TotalDifficulty = TotalDifficulty.Uint64()
	ZondNew.Result.Uncles = Zond.PreResult.Uncles
	ZondNew.Result.Withdrawals = Zond.PreResult.Withdrawals
	ZondNew.Result.WithdrawalsRoot = Zond.PreResult.WithdrawalsRoot

	return ZondNew
}

// func ConvertModelsUint64(Zond models.Zond) models.ZondUint64Version {
// 	BaseFeePerGas := new(big.Int)
// 	BaseFeePerGas.SetString(Zond.PreResult.BaseFeePerGas[2:], 16)
// 	BaseFeePerGas.Uint64()

// 	GasLimit := new(big.Int)
// 	GasLimit.SetString(Zond.PreResult.GasLimit[2:], 16)
// 	GasLimit.Uint64()

// 	GasUsed := new(big.Int)
// 	GasUsed.SetString(Zond.PreResult.GasUsed[2:], 16)
// 	GasUsed.Uint64()

// 	blockNumber := new(big.Int)
// 	blockNumber.SetString(Zond.PreResult.Number[2:], 16)
// 	blockNumber.Uint64()

// 	Timestamp := new(big.Int)
// 	Timestamp.SetString(Zond.PreResult.Timestamp[2:], 16)
// 	Timestamp.Uint64()

// 	var ZondNew models.ZondUint64Version

// 	ZondNew.Jsonrpc = Zond.Jsonrpc
// 	ZondNew.ID = Zond.ID
// 	ZondNew.Result.BaseFeePerGas = BaseFeePerGas.Uint64()
// 	ZondNew.Result.GasLimit = GasLimit.Uint64()
// 	ZondNew.Result.GasUsed = GasUsed.Uint64()
// 	ZondNew.Result.Hash = Zond.PreResult.Hash
// 	ZondNew.Result.Number = blockNumber.Uint64()
// 	ZondNew.Result.ParentHash = Zond.PreResult.ParentHash
// 	ZondNew.Result.ReceiptsRoot = Zond.PreResult.ReceiptsRoot
// 	ZondNew.Result.StateRoot = Zond.PreResult.StateRoot
// 	ZondNew.Result.Timestamp = Timestamp.Uint64()
// 	ZondNew.Result.Transactions = Zond.PreResult.Transactions
// 	for index, _ := range ZondNew.Result.Transactions {
// 		_, status, err := rpc.GetContractAddress(Zond.PreResult.Transactions[index].Hash)
// 		if err != nil {
// 			configs.Logger.Warn("Failed to do rpc request: ", zap.Error(err))
// 		}
// 		ZondNew.Result.Transactions[index].Status = status
// 	}
// 	ZondNew.Result.TransactionsRoot = Zond.PreResult.TransactionsRoot
// 	return ZondNew
// }
