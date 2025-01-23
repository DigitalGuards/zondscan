package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
	"context"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const syncStateCollection = "sync_state"

// GetLastKnownBlockNumber retrieves the last successfully synced block number
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
		configs.Logger.Warn("Failed to get last known block number", zap.Error(err))
		return "0x0"
	}

	return result.BlockNumber
}

// StoreLastKnownBlockNumber stores the last successfully synced block number
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

func GetLatestBlockFromDB() *models.ZondDatabaseBlock {
	if !IsCollectionsExist() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{}
	options := options.FindOne().SetProjection(bson.M{"result.number": 1, "result.timestamp": 1}).SetSort(bson.M{"result.number": -1})

	var Zond models.ZondDatabaseBlock

	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&Zond)

	if err != nil {
		configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
	}

	return &Zond
}

func GetLatestBlockNumberFromDB() string {
	if !IsCollectionsExist() {
		return "0x0"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{}
	options := options.FindOne().SetProjection(bson.M{"result.number": 1}).SetSort(bson.M{"result.number": -1})

	var Zond models.ZondDatabaseBlock

	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&Zond)

	if err != nil {
		configs.Logger.Warn("Failed to get latest block number", zap.Error(err))
		// Try to get the last known block number from a persistent store
		lastKnown := GetLastKnownBlockNumber()
		if lastKnown != "0x0" {
			configs.Logger.Info("Using last known block number", zap.String("block", lastKnown))
			return lastKnown
		}
		return "0x0"
	}

	// Store this successful block number
	StoreLastKnownBlockNumber(Zond.Result.Number)
	return Zond.Result.Number
}

func GetLatestBlockHashHeaderFromDB(blockNumber string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"result.number": blockNumber}
	options := options.FindOne().SetProjection(bson.M{"result.hash": 1})

	var Zond models.ZondDatabaseBlock

	err := configs.BlocksCollections.FindOne(ctx, filter, options).Decode(&Zond)

	if err != nil {
		configs.Logger.Info("Failed to do FindOne in the blocks collection", zap.Error(err))
		return ""
	}

	return Zond.Result.Hash
}

func InsertBlockDocument(obj models.ZondDatabaseBlock) {
	hashField := obj.Result.Hash

	if len(hashField) > 0 {
		result, err := configs.BlocksCollections.InsertOne(context.TODO(), obj)
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

func Rollback(blockNumber string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Get the block's timestamp before deleting it
	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.number": blockNumber}).Decode(&block)
	if err != nil {
		configs.Logger.Warn("Failed to find block for rollback: ", zap.Error(err))
		return
	}
	timestamp := block.Result.Timestamp

	// Delete the block
	_, err = configs.BlocksCollections.DeleteOne(ctx, bson.M{"result.number": blockNumber})
	if err != nil {
		configs.Logger.Warn("Failed to delete in the blocks collection: ", zap.Error(err))
		return
	}

	// Clean up transactions for this block
	_, err = configs.TransactionByAddressCollections.DeleteMany(ctx, bson.M{"blockTimestamp": timestamp})
	if err != nil {
		configs.Logger.Warn("Failed to clean up transactions: ", zap.Error(err))
	}

	// Clean up transfers for this block
	_, err = configs.TransferCollections.DeleteMany(ctx, bson.M{"blockTimestamp": timestamp})
	if err != nil {
		configs.Logger.Warn("Failed to clean up transfers: ", zap.Error(err))
	}

	// Clean up internal transactions for this block
	_, err = configs.InternalTransactionByAddressCollections.DeleteMany(ctx, bson.M{"blockTimestamp": timestamp})
	if err != nil {
		configs.Logger.Warn("Failed to clean up internal transactions: ", zap.Error(err))
	}

	// Update addresses collection
	var transfers []models.Transfer
	cursor, err := configs.TransferCollections.Find(ctx, bson.M{"blockTimestamp": timestamp})
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var transfer models.Transfer
			if err = cursor.Decode(&transfer); err == nil {
				transfers = append(transfers, transfer)
			}
		}

		// Update balances for affected addresses
		for _, transfer := range transfers {
			if transfer.From != "" {
				updateAddressBalance(transfer.From)
			}
			if transfer.To != "" {
				updateAddressBalance(transfer.To)
			}
		}
	}

	configs.Logger.Info("Successfully rolled back block and cleaned up related collections",
		zap.String("block", blockNumber),
		zap.String("timestamp", timestamp))
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
