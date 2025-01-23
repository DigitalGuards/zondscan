package db

import (
	"Zond2mongoDB/configs"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

func IsCollectionsExist() bool {
	cNames := configs.GetListCollectionNames(configs.DB)

	collectionsExist := true
	if len(cNames) == 0 {
		collectionsExist = false
	}

	return collectionsExist
}

func CountWallets() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := configs.AddressesCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		configs.Logger.Info("Failed to find wallet count document", zap.Error(err))
		return 0
	}

	return results
}
