package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
	"Zond2mongoDB/validation"
	"context"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const syncStateCollection = "sync_state"

func GetLatestBlockFromDB() *models.ZondDatabaseBlock {
	if !IsCollectionsExist() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{}
	findOptions := options.FindOne().SetProjection(bson.M{"result.number": 1, "result.timestamp": 1}).SetSort(bson.M{"result.number": -1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter, findOptions).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
	}

	return &block
}

func GetLatestBlockNumberFromDB() string {
	if !IsCollectionsExist() {
		configs.Logger.Info("No collections exist yet")
		return "0x0"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.BlocksCollections.CountDocuments(ctx, bson.D{})
	if err != nil {
		configs.Logger.Warn("Failed to count blocks", zap.Error(err))
		return "0x0"
	}

	if count == 0 {
		configs.Logger.Info("No blocks in database")
		return "0x0"
	}

	filter := bson.D{}
	findOptions := options.FindOne().SetProjection(bson.M{"result.number": 1}).SetSort(bson.M{"result.number": -1})

	var block models.ZondDatabaseBlock
	err = configs.BlocksCollections.FindOne(ctx, filter, findOptions).Decode(&block)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("No blocks found in database")
		} else {
			configs.Logger.Warn("Failed to get latest block number", zap.Error(err))
		}
		return "0x0"
	}

	if block.Result.Number == "" {
		configs.Logger.Warn("Found block but number is empty")
		return "0x0"
	}

	configs.Logger.Info("Found latest block in database",
		zap.String("block", block.Result.Number))
	return block.Result.Number
}

func GetLatestBlockHashHeaderFromDB(blockNumber string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.number": blockNumber}
	findOptions := options.FindOne().SetProjection(bson.M{"result.hash": 1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, filter, findOptions).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
		return ""
	}

	return block.Result.Hash
}

func InsertBlockDocument(block models.ZondDatabaseBlock) {
	hashField := block.Result.Hash
	if len(hashField) > 0 {
		result, err := configs.BlocksCollections.InsertOne(context.TODO(), block)
		if err != nil {
			configs.Logger.Warn("Failed to insert in the blocks collection: ", zap.Error(err))
		}
		_ = result
	}
}

func InsertManyBlockDocuments(blocks []interface{}) {
	_, err := configs.BlocksCollections.InsertMany(context.TODO(), blocks)
	if err != nil {
		configs.Logger.Warn("Failed to insert many block documents", zap.Error(err))
	}
}

func GetLastKnownBlockNumber() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	syncColl := configs.GetCollection(configs.DB, syncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{
		"_id": "last_synced_block",
	}).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("No sync state found, this appears to be the first run")
		} else {
			configs.Logger.Warn("Failed to get last known block number", zap.Error(err))
		}
		return "0x0"
	}

	if result.BlockNumber == "" {
		configs.Logger.Warn("Found sync state but block number is empty")
		return "0x0"
	}

	configs.Logger.Info("Found last known block in sync state",
		zap.String("block", result.BlockNumber))
	return result.BlockNumber
}

func StoreLastKnownBlockNumber(blockNumber string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, syncStateCollection)
	_, err := syncColl.UpdateOne(
		ctx,
		bson.M{"_id": "last_synced_block"},
		bson.M{"$set": bson.M{"block_number": blockNumber}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		configs.Logger.Warn("Failed to store last known block number",
			zap.String("block", blockNumber),
			zap.Error(err))
	}
}

func Rollback(blockNumber string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Validate block number format
	if !validation.IsValidHexString(blockNumber) {
		configs.Logger.Error("Invalid block number format for rollback",
			zap.String("block", blockNumber))
		return
	}

	// Get the block's data before deleting it
	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.number": blockNumber}).Decode(&block)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("Block not found for rollback, skipping",
				zap.String("block", blockNumber))
			return
		}
		configs.Logger.Warn("Failed to find block for rollback: ", zap.Error(err))
		return
	}

	blockHash := block.Result.Hash
	if blockHash == "" || !validation.IsValidHexString(blockHash) {
		configs.Logger.Error("Invalid block hash for rollback",
			zap.String("block", blockNumber),
			zap.String("hash", blockHash))
		return
	}

	// Start a session for atomic operations
	session, err := configs.DB.StartSession()
	if err != nil {
		configs.Logger.Error("Failed to start session for rollback", zap.Error(err))
		return
	}
	defer session.EndSession(ctx)

	// Track affected addresses for balance recalculation
	affectedAddresses := make(map[string]bool)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Delete the block
		result, err := configs.BlocksCollections.DeleteOne(sessCtx, bson.M{
			"result.number": blockNumber,
			"result.hash":   blockHash,
		})
		if err != nil {
			return nil, err
		}
		if result.DeletedCount == 0 {
			configs.Logger.Info("No block found to delete",
				zap.String("block", blockNumber))
			return nil, nil
		}

		// Clean up and track addresses from transactions
		txCursor, err := configs.TransactionByAddressCollections.Find(sessCtx, bson.M{
			"BlockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err == nil {
			defer txCursor.Close(sessCtx)
			for txCursor.Next(sessCtx) {
				var tx struct {
					Address string `bson:"Address"`
				}
				if err = txCursor.Decode(&tx); err == nil && tx.Address != "" {
					affectedAddresses[tx.Address] = true
				}
			}
		}

		// Delete transactions
		_, err = configs.TransactionByAddressCollections.DeleteMany(sessCtx, bson.M{
			"BlockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err != nil {
			return nil, err
		}

		// Clean up and track addresses from transfers
		transferCursor, err := configs.TransferCollections.Find(sessCtx, bson.M{
			"blockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err == nil {
			defer transferCursor.Close(sessCtx)
			for transferCursor.Next(sessCtx) {
				var transfer models.Transfer
				if err = transferCursor.Decode(&transfer); err == nil {
					if transfer.From != "" {
						affectedAddresses[transfer.From] = true
					}
					if transfer.To != "" {
						affectedAddresses[transfer.To] = true
					}
				}
			}
		}

		// Delete transfers
		_, err = configs.TransferCollections.DeleteMany(sessCtx, bson.M{
			"blockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err != nil {
			return nil, err
		}

		// Clean up and track addresses from internal transactions
		internalTxCursor, err := configs.InternalTransactionByAddressCollections.Find(sessCtx, bson.M{
			"BlockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err == nil {
			defer internalTxCursor.Close(sessCtx)
			for internalTxCursor.Next(sessCtx) {
				var tx struct {
					Address string `bson:"Address"`
				}
				if err = internalTxCursor.Decode(&tx); err == nil && tx.Address != "" {
					affectedAddresses[tx.Address] = true
				}
			}
		}

		// Delete internal transactions
		_, err = configs.InternalTransactionByAddressCollections.DeleteMany(sessCtx, bson.M{
			"BlockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err != nil {
			return nil, err
		}

		// Clean up contract-related data
		_, err = configs.ContractCodeCollection.DeleteMany(sessCtx, bson.M{
			"BlockNumber": blockNumber,
			"blockHash":   blockHash,
		})
		if err != nil {
			return nil, err
		}

		// Only update sync state if we actually rolled back something
		if result.DeletedCount > 0 {
			prevBlock := utils.SubtractHexNumbers(blockNumber, "0x1")
			_, err = configs.GetCollection(configs.DB, syncStateCollection).UpdateOne(
				sessCtx,
				bson.M{"_id": "last_synced_block"},
				bson.M{"$set": bson.M{"block_number": prevBlock}},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				return nil, err
			}
		} else {
			// If block doesn't exist, update sync state to last valid block
			lastValidBlock := utils.SubtractHexNumbers(blockNumber, "0x1")
			_, err = configs.GetCollection(configs.DB, syncStateCollection).UpdateOne(
				sessCtx,
				bson.M{"_id": "last_synced_block"},
				bson.M{"$set": bson.M{"block_number": lastValidBlock}},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	if err != nil {
		configs.Logger.Error("Failed to execute rollback transaction",
			zap.String("block", blockNumber),
			zap.Error(err))
		return
	}

	// Recalculate balances for all affected addresses
	addressCount := 0
	for address := range affectedAddresses {
		updateAddressBalance(address)
		addressCount++
	}

	configs.Logger.Info("Successfully rolled back block and cleaned up related collections",
		zap.String("block", blockNumber),
		zap.String("hash", blockHash),
		zap.Int("addresses_updated", addressCount))
}

func updateAddressBalance(address string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all transfers for this address
	cursor, err := configs.TransferCollections.Find(ctx, bson.M{
		"$or": []bson.M{
			{"from": address},
			{"to": address},
		},
	})
	if err != nil {
		configs.Logger.Warn("Failed to find transfers for address: ", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	// Calculate new balance using big.Int for hex values
	balance := new(big.Int)
	for cursor.Next(ctx) {
		var transfer models.Transfer
		if err = cursor.Decode(&transfer); err != nil {
			continue
		}

		value := utils.HexToInt(transfer.Value)
		if transfer.From == address {
			balance.Sub(balance, value)
		}
		if transfer.To == address {
			balance.Add(balance, value)
		}
	}

	// Convert balance to hex string
	balanceHex := "0x" + balance.Text(16)
	if balance.Sign() == 0 {
		balanceHex = "0x0"
	}

	// Update address balance
	opts := options.Update().SetUpsert(true)
	_, err = configs.AddressesCollections.UpdateOne(
		ctx,
		bson.M{"id": address},
		bson.M{"$set": bson.M{"balance": balanceHex}},
		opts,
	)
	if err != nil {
		configs.Logger.Warn("Failed to update address balance: ", zap.Error(err))
	}
}
