package models

import "time"

// MarketDataResponse represents the response from CoinGecko API
type MarketDataResponse struct {
	MarketData struct {
		MarketCap struct {
			USD float32 `json:"usd" bson:"usd" validate:"required"`
		} `json:"market_cap" bson:"market_cap"`
		CurrentPrice struct {
			USD float32 `json:"usd" bson:"usd" validate:"required"`
		} `json:"current_price" bson:"current_price"`
	} `json:"market_data" bson:"market_data"`
	LastUpdated time.Time `json:"last_updated" bson:"last_updated"`
}

// CoinGeckoDocument represents how we store the data in MongoDB
type CoinGeckoDocument struct {
	MarketCapUSD float32   `bson:"marketCapUSD" json:"marketCapUSD"`
	PriceUSD     float32   `bson:"priceUSD" json:"priceUSD"`
	LastUpdated  time.Time `bson:"lastUpdated" json:"lastUpdated"`
}

// ToDocument converts API response to MongoDB document format
func (m *MarketDataResponse) ToDocument() *CoinGeckoDocument {
	return &CoinGeckoDocument{
		MarketCapUSD: m.MarketData.MarketCap.USD,
		PriceUSD:     m.MarketData.CurrentPrice.USD,
		LastUpdated:  m.LastUpdated,
	}
}
