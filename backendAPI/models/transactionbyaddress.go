package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TransactionByAddress struct {
	ID          primitive.ObjectID `bson:"_id"`
	InOut       uint64             `json:"InOut"`
	TxType      uint32             `json:"TxType"`
	Address     string             `json:"Address" bson:"Address"`
	From        string             `json:"From" bson:"From"`
	To          string             `json:"To" bson:"To"`
	TxHash      string             `json:"TxHash" bson:"TxHash"`
	TimeStamp   uint64             `json:"TimeStamp"`
	Amount      float32            `json:"Amount"`
	PaidFees    float32            `json:"PaidFees"`
	BlockNumber uint64             `json:"BlockNumber"`
}
