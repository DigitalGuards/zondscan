package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Transfer struct {
	ID             primitive.ObjectID `bson:"_id"`
	BlockNumber    string             `bson:"blockNumber" json:"blockNumber"`       // hex string
	BlockTimestamp string             `bson:"blockTimestamp" json:"blockTimestamp"` // hex string
	From           string             `bson:"from" json:"from"`
	To             string             `bson:"to" json:"to"`
	TxHash         string             `bson:"txHash" json:"txHash"`
	Pk             string             `bson:"pk" json:"pk"`
	Signature      string             `bson:"signature" json:"signature"`
	Nonce          string             `bson:"nonce" json:"nonce"`   // hex string
	Value          float64            `bson:"value" json:"value"`   // QRL amount as float
	Status         string             `bson:"status" json:"status"` // hex string
	Size           string             `bson:"size" json:"size"`     // hex string
}
