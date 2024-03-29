package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type CoinGecko struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	MarketCapUSD float64            `bson:"marketCapUSD"`
}
