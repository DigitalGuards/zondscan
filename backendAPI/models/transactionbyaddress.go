package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TransactionByAddress struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	InOut       int               `bson:"inOut" json:"InOut"`
	TxType      string            `bson:"txType" json:"TxType"`
	Address     string             `json:"Address" bson:"Address"`
	From        string            `bson:"from" json:"From"`
	To          string            `bson:"to" json:"To"`
	TxHash      string            `bson:"txHash" json:"TxHash"`
	TimeStamp   string            `bson:"timeStamp" json:"TimeStamp"`
	Amount      float32           `bson:"amount" json:"Amount"`
	PaidFees    float32           `bson:"paidFees" json:"PaidFees"`
	BlockNumber uint64             `json:"BlockNumber"`
}
