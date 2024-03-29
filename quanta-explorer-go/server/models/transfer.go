package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Transfer struct {
	ID              primitive.ObjectID `bson:"_id"`
	BlockNumber     uint64             `json:"blockNumber"`
	BlockTimestamp  uint64             `json:"blockTimestamp"`
	From          	[]byte             `json:"from"`
	To              []byte             `json:"to"`
	TxHash       	[]byte             `json:"txHash"`
	Pk           	[]byte             `json:"pk"`
	Signature       []byte             `json:"signature"`
	Nonce 			uint64             `json:"nonce"`
	Value           uint64             `json:"value"`
	Status          uint32             `json:"status"`
	Size          	uint64             `json:"size"`
}

type TransactionsVolume struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Volume int64              `bson:"volume"`
}
