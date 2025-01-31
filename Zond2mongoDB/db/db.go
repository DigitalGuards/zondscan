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

	// Only count addresses that are not contracts
	filter := bson.M{"isContract": false}
	results, err := configs.AddressesCollections.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to count wallets", zap.Error(err))
		return 0
	}

	configs.Logger.Debug("Counted wallets",
		zap.Int64("total_non_contract_addresses", results))

	return results
}
