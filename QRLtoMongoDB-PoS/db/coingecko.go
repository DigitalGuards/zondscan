package db

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/fetch"
	"QRLtoMongoDB-PoS/models"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func UpdateCoinGeckoDataInDB(data *models.MarketDataResponse) error {
	if data == nil {
		return errors.New("cannot update database with nil data")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updateData := map[string]interface{}{
		"marketCapUSD": data.MarketData.MarketCap.USD,
	}

	_, err := configs.CoinGeckoCollections.UpdateOne(ctx, map[string]interface{}{}, map[string]interface{}{"$set": updateData}, options.Update().SetUpsert(true))

	if err != nil {
		configs.Logger.Error("Failed to update the coingecko collection", zap.Error(err))
		return err
	}

	return nil
}

func PeriodicallyUpdateCoinGeckoData() {
	data, err := fetch.FetchCoinGeckoData()
	if err != nil {
		configs.Logger.Error("Error fetching data from CoinGecko",
			zap.Error(err),
			zap.String("source", "CoinGecko API"))
		return // Return early on fetch error to prevent nil pointer dereference
	}

	if data == nil {
		configs.Logger.Error("Received nil data from CoinGecko API")
		return
	}

	err = UpdateCoinGeckoDataInDB(data)
	if err != nil {
		configs.Logger.Error("Error updating MongoDB with CoinGecko data",
			zap.Error(err),
			zap.String("source", "MongoDB"))
	}
}
