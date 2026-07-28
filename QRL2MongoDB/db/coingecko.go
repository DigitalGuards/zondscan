package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/fetch"
	"QRL2MongoDB/models"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func UpdateCoinGeckoDataInDB(data *models.MarketDataResponse) error {
	if data == nil {
		return errors.New("cannot update database with nil data")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert API response to database document
	data.LastUpdated = time.Now()
	doc := data.ToDocument()

	update := primitive.D{{Key: "$set", Value: doc}}
	opts := options.Update().SetUpsert(true)

	_, err := configs.CoinGeckoCollections.UpdateOne(ctx, primitive.D{}, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update the coingecko collection",
			zap.Error(err),
			zap.Float32("marketCap", doc.MarketCapUSD),
			zap.Float32("price", doc.PriceUSD))
		return err
	}

	configs.Logger.Info("Successfully updated CoinGecko data",
		zap.Float32("marketCap", doc.MarketCapUSD),
		zap.Float32("price", doc.PriceUSD),
		zap.Float32("volume", doc.VolumeUSD),
		zap.Time("lastUpdated", doc.LastUpdated))

	// Also store in price history for historical tracking
	if err := InsertPriceHistory(data); err != nil {
		configs.Logger.Warn("Failed to insert price history", zap.Error(err))
		// Don't return error - current price update succeeded
	}

	return nil
}

// InsertPriceHistory adds a new price snapshot to the price history collection
func InsertPriceHistory(data *models.MarketDataResponse) error {
	if data == nil {
		return errors.New("cannot insert nil data to price history")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	historyDoc := data.ToPriceHistoryDocument()

	_, err := configs.PriceHistoryCollections.InsertOne(ctx, historyDoc)
	if err != nil {
		configs.Logger.Error("Failed to insert price history",
			zap.Error(err),
			zap.Float32("price", historyDoc.PriceUSD),
			zap.Time("timestamp", historyDoc.Timestamp))
		return err
	}

	configs.Logger.Debug("Inserted price history snapshot",
		zap.Float32("price", historyDoc.PriceUSD),
		zap.Float32("volume", historyDoc.VolumeUSD),
		zap.Time("timestamp", historyDoc.Timestamp))

	return nil
}

func PeriodicallyUpdateCoinGeckoData() {
	data, err := fetch.FetchCoinGeckoData()
	if err != nil {
		configs.Logger.Error("Error fetching data from CoinGecko",
			zap.Error(err),
			zap.String("source", "CoinGecko API"))
		return
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
