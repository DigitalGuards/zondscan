package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TokenBalance represents a holder's balance for a specific token
type TokenBalance struct {
	ID              primitive.ObjectID `bson:"_id"`
	ContractAddress string            `bson:"contractAddress" json:"contractAddress"`
	HolderAddress   string            `bson:"holderAddress" json:"holderAddress"`
	Balance         string            `bson:"balance" json:"balance"`         // hex string
	BlockNumber     string            `bson:"blockNumber" json:"blockNumber"` // hex string
	UpdatedAt       string            `bson:"updatedAt" json:"updatedAt"`
}
