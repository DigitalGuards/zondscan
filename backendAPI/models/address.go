package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Address struct {
	ObjectId primitive.ObjectID `bson:"_id"`
	ID       []byte             `json:"id"`
	Balance  float32            `json:"balance"`
	Nonce    uint64             `json:"nonce"`
}
