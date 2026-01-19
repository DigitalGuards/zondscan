package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoinGecko struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	MarketCapUSD float64            `bson:"marketCapUSD"`
	PriceUSD     float64            `bson:"priceUSD"`
	VolumeUSD    float64            `bson:"volumeUSD"`
	LastUpdated  time.Time          `bson:"lastUpdated"`
}

// PriceHistory represents a historical price snapshot
type PriceHistory struct {
	Timestamp    time.Time `bson:"timestamp" json:"timestamp"`
	PriceUSD     float64   `bson:"priceUSD" json:"priceUSD"`
	MarketCapUSD float64   `bson:"marketCapUSD" json:"marketCapUSD"`
	VolumeUSD    float64   `bson:"volumeUSD" json:"volumeUSD"`
}

// PriceHistoryResponse is the API response for price history
type PriceHistoryResponse struct {
	Data     []PriceHistory `json:"data"`
	Interval string         `json:"interval"`
	Count    int            `json:"count"`
}
