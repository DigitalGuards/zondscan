package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Transfer struct {
	ID             primitive.ObjectID `bson:"_id"`
	BlockNumber    string             `json:"blockNumber"`    // hex string
	BlockTimestamp string             `json:"blockTimestamp"` // hex string
	From           string             `json:"from"`
	To             string             `json:"to"`
	TxHash         string             `json:"txHash"`
	Pk             string             `json:"pk"`
	Signature      string             `json:"signature"`
	Nonce          string             `json:"nonce"`  // hex string
	Value          string             `json:"value"`  // hex string
	Status         string             `json:"status"` // hex string
	Size           string             `json:"size"`   // hex string
}
