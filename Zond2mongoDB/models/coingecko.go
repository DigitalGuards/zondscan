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
		TotalVolume struct {
			USD float32 `json:"usd" bson:"usd"`
		} `json:"total_volume" bson:"total_volume"`
	} `json:"market_data" bson:"market_data"`
	LastUpdated time.Time `json:"last_updated" bson:"last_updated"`
}

// CoinGeckoDocument represents how we store the current data in MongoDB
type CoinGeckoDocument struct {
	MarketCapUSD float32   `bson:"marketCapUSD" json:"marketCapUSD"`
	PriceUSD     float32   `bson:"priceUSD" json:"priceUSD"`
	VolumeUSD    float32   `bson:"volumeUSD" json:"volumeUSD"`
	LastUpdated  time.Time `bson:"lastUpdated" json:"lastUpdated"`
}

// PriceHistoryDocument represents a historical price snapshot
type PriceHistoryDocument struct {
	Timestamp    time.Time `bson:"timestamp" json:"timestamp"`
	PriceUSD     float32   `bson:"priceUSD" json:"priceUSD"`
	MarketCapUSD float32   `bson:"marketCapUSD" json:"marketCapUSD"`
	VolumeUSD    float32   `bson:"volumeUSD" json:"volumeUSD"`
}

// ToDocument converts API response to MongoDB document format
func (m *MarketDataResponse) ToDocument() *CoinGeckoDocument {
	return &CoinGeckoDocument{
		MarketCapUSD: m.MarketData.MarketCap.USD,
		PriceUSD:     m.MarketData.CurrentPrice.USD,
		VolumeUSD:    m.MarketData.TotalVolume.USD,
		LastUpdated:  m.LastUpdated,
	}
}

// ToPriceHistoryDocument converts API response to price history document
func (m *MarketDataResponse) ToPriceHistoryDocument() *PriceHistoryDocument {
	return &PriceHistoryDocument{
		Timestamp:    m.LastUpdated,
		PriceUSD:     m.MarketData.CurrentPrice.USD,
		MarketCapUSD: m.MarketData.MarketCap.USD,
		VolumeUSD:    m.MarketData.TotalVolume.USD,
	}
}
