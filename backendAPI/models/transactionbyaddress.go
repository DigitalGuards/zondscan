package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TransactionByAddress struct {
	ID          primitive.ObjectID `bson:"_id"`
	InOut       uint64             `json:"InOut"`
	TxType      uint32             `json:"TxType"`
	Address     []byte             `json:"Address"`
	From        []byte             `json:"From"`
	To          []byte             `json:"To"`
	TxHash      []byte             `json:"TxHash"`
	TimeStamp   uint64             `json:"TimeStamp"`
	Amount      float32            `json:"Amount"`
	PaidFees    float32            `json:"PaidFees"`
	BlockNumber uint64             `json:"BlockNumber"`
}
