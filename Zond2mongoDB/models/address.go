package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Address struct {
	ObjectId primitive.ObjectID `bson:"_id"`
	ID       string             `json:"id"` // Store as hex string
	Balance  float32            `json:"balance"`
	Nonce    uint64             `json:"nonce"`
}
