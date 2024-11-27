package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
