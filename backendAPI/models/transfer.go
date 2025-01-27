package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Transfer struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	BlockNumber    string             `bson:"blockNumber"`
	BlockTimestamp string             `bson:"blockTimestamp"`
	From           string             `bson:"from"`
	To             string             `bson:"to"`
	TxHash         string             `bson:"txHash"`
	Value          string             `bson:"value"`
	GasUsed        string             `bson:"gasUsed"`
	GasPrice       string             `bson:"gasPrice"`
	Nonce          string             `bson:"nonce"`
	Signature      string             `bson:"signature"`
	Pk             string             `bson:"pk"`
	Size           string             `bson:"size"`
}

type TransactionsVolume struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Volume int64              `bson:"volume"`
}
