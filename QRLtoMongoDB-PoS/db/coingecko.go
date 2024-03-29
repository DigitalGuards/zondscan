package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/fetch"
	"QRLtoMongoDB-PoS/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func UpdateCoinGeckoDataInDB(data *models.MarketDataResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updateData := map[string]interface{}{
		"marketCapUSD": data.MarketData.MarketCap.USD,
	}

	_, err := configs.CoinGeckoCollections.UpdateOne(ctx, map[string]interface{}{}, map[string]interface{}{"$set": updateData}, options.Update().SetUpsert(true))

	if err != nil {
		configs.Logger.Warn("Failed to update the coingecko collection: ", zap.Error(err))
	}

	return err
}

func PeriodicallyUpdateCoinGeckoData() {
	data, err := fetch.FetchCoinGeckoData()
	if err != nil {
		configs.Logger.Warn("Error fetching data from CoinGecko: ", zap.Error(err))
	}

	err = UpdateCoinGeckoDataInDB(data)
	if err != nil {
		configs.Logger.Warn("Error updating MongoDB with CoinGecko data: ", zap.Error(err))
	}
}
