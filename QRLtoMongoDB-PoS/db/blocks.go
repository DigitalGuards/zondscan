package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"context"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	_, err := configs.BlocksCollections.DeleteOne(ctx, bson.M{"result.number": number})

	if err != nil {
		configs.Logger.Warn("Failed to delete in the blocks collection: ", zap.Error(err))
		return
	}

	configs.Logger.Info("Succesfully deleted: ", zap.String("Block number:", strconv.Itoa(int(number))))
}
