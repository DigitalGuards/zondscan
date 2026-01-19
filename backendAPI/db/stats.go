package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnDateTime() string {
	currentTime := time.Now()
	return currentTime.Format("2006-01-02 3:4:5 PM")
}

func ReturnTotalCirculatingSupply() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CirculatingSupply

	err := configs.TotalCirculatingSupplyCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return result.Circulating
}

func GetMarketCap() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CoinGecko

	err := configs.CoinGeckoCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.MarketCapUSD
}

func GetCurrentPrice() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CoinGecko

	err := configs.CoinGeckoCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.PriceUSD
}

// GetCurrentVolume returns the current 24h trading volume in USD
func GetCurrentVolume() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CoinGecko

	err := configs.CoinGeckoCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.VolumeUSD
}

// GetPriceHistory returns historical price data for the given duration
// interval can be: "4h", "12h", "24h", "7d", "30d", "all"
func GetPriceHistory(interval string) ([]models.PriceHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Calculate the time range based on interval
	var since time.Time
	var limit int64

	now := time.Now()
	switch interval {
	case "4h":
		since = now.Add(-4 * time.Hour)
		limit = 8 // ~30 min intervals
	case "12h":
		since = now.Add(-12 * time.Hour)
		limit = 24 // ~30 min intervals
	case "24h":
		since = now.Add(-24 * time.Hour)
		limit = 48 // ~30 min intervals
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
		limit = 336 // ~30 min intervals for 7 days
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
		limit = 1440 // ~30 min intervals for 30 days
	case "all":
		since = time.Time{} // Beginning of time
		limit = 0           // No limit
	default:
		// Default to 24h
		since = now.Add(-24 * time.Hour)
		limit = 48
	}

	// Build filter
	filter := bson.M{}
	if !since.IsZero() {
		filter["timestamp"] = bson.M{"$gte": since}
	}

	// Sort by timestamp descending (most recent first)
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}

	cursor, err := configs.PriceHistoryCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query price history: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.PriceHistory
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode price history: %w", err)
	}

	// Reverse to return oldest first (chronological order)
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results, nil
}
