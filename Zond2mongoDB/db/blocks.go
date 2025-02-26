package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
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

func StoreLastKnownBlockNumber(blockNumber string) error {
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
		return err
	}
	return nil
}

// GetLastSyncedBlock retrieves the last synced block from the database
func GetLastSyncedBlock() (*models.ZondDatabaseBlock, error) {
	// First check if we have a sync state document
	collection := configs.GetCollection(configs.DB, "syncState")
	ctx := context.Background()

	var result struct {
		BlockNumber string `bson:"blockNumber"`
	}

	err := collection.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("No sync state found, starting from genesis block")
			// Return a block with number "0x0" to start from genesis
			return &models.ZondDatabaseBlock{
				Result: models.Result{
					Number: "0x0",
				},
			}, nil
		}
		return nil, err
	}

	if result.BlockNumber == "" {
		configs.Logger.Warn("Found sync state but block number is empty")
		return nil, nil
	}

	// Create a ZondDatabaseBlock object with the retrieved block number
	block := &models.ZondDatabaseBlock{
		Result: models.Result{
			Number: result.BlockNumber,
		},
	}

	configs.Logger.Info("Found last synced block in sync state",
		zap.String("block", result.BlockNumber))
	return block, nil
}

func Rollback(blockNumber string) error {
	ctx := context.Background()

	// Find all blocks after the given block number
	filter := bson.M{
		"result.number": bson.M{
			"$gt": blockNumber,
		},
	}

	// Get blocks to be removed for logging
	cursor, err := configs.BlocksCollections.Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to find blocks for rollback",
			zap.String("from_block", blockNumber),
			zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	var blocks []models.ZondDatabaseBlock
	if err = cursor.All(ctx, &blocks); err != nil {
		configs.Logger.Error("Failed to decode blocks for rollback",
			zap.Error(err))
		return err
	}

	// Log blocks being removed
	for _, block := range blocks {
		configs.Logger.Info("Rolling back block",
			zap.String("number", block.Result.Number),
			zap.String("hash", block.Result.Hash))
	}

	// Delete blocks in a transaction
	session, err := configs.DB.StartSession()
	if err != nil {
		configs.Logger.Error("Failed to start session for rollback",
			zap.Error(err))
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Delete blocks
		_, err := configs.BlocksCollections.DeleteMany(sessCtx, filter)
		if err != nil {
			return nil, err
		}

		// Update sync state
		err = StoreLastKnownBlockNumber(blockNumber)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		configs.Logger.Error("Failed to execute rollback transaction",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully rolled back to block",
		zap.String("block_number", blockNumber))
	return nil
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
