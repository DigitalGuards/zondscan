package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"context"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

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
		panic(err)
	}
}

func Rollback(number uint64) {
	var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Get the block's timestamp before deleting it
	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.number": number}).Decode(&block)
	if err != nil {
		configs.Logger.Warn("Failed to find block for rollback: ", zap.Error(err))
		return
	}
	timestamp := block.Result.Timestamp

	// Delete the block
	_, err = configs.BlocksCollections.DeleteOne(ctx, bson.M{"result.number": number})
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
	// This is more complex as we need to recalculate balances
	// We'll trigger a balance recalculation for affected addresses
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
			if transfer.From != nil {
				updateAddressBalance(transfer.From)
			}
			if transfer.To != nil {
				updateAddressBalance(transfer.To)
			}
		}
	}

	configs.Logger.Info("Successfully rolled back block and cleaned up related collections: ",
		zap.String("Block number:", strconv.Itoa(int(number))),
		zap.Uint64("timestamp", timestamp))
}

func updateAddressBalance(address []byte) {
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

	// Calculate new balance
	var balance uint64
	for cursor.Next(ctx) {
		var transfer models.Transfer
		if err = cursor.Decode(&transfer); err != nil {
			continue
		}

		if transfer.From != nil && string(transfer.From) == string(address) {
			balance -= transfer.Value
		}
		if transfer.To != nil && string(transfer.To) == string(address) {
			balance += transfer.Value
		}
	}

	// Update address balance
	opts := options.Update().SetUpsert(true)
	_, err = configs.AddressesCollections.UpdateOne(
		ctx,
		bson.M{"id": address},
		bson.M{"$set": bson.M{"balance": balance}},
		opts,
	)
	if err != nil {
		configs.Logger.Warn("Failed to update address balance: ", zap.Error(err))
	}
}
