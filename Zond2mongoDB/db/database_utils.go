package db

import (
	"Zond2mongoDB/configs"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// IsCollectionsExist checks if MongoDB collections exist in the database.
// This is primarily used during initialization to determine if we need to
// bootstrap the database with initial collections.
// Returns true if collections are found, false if the database is empty.
func IsCollectionsExist() bool {
	cNames := configs.GetListCollectionNames(configs.DB)

	collectionsExist := true
	if len(cNames) == 0 {
		collectionsExist = false
	}

	return collectionsExist
}

// CountWallets returns the total number of non-contract wallet addresses in the database.
// This function is used for analytics purposes and by the wallet count sync service.
// Returns 0 if an error occurs during counting.
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
