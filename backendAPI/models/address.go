package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Address struct {
	ObjectId primitive.ObjectID `bson:"_id"`
	ID       string             `json:"id"` // Changed from []byte to string
	Balance  float64            `json:"balance"`
	Nonce    uint64             `json:"nonce"`
}
