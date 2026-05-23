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
	// Input is the transaction's calldata (the tx's `data` field on the
	// block). Populated by ReturnSingleTransfer from the blocks collection.
	// Empty for plain QRL transfers, "0x" + selector + ABI args for contract
	// calls. Drives the Input Data card + Event Logs panel on the tx page.
	Input string `bson:"input,omitempty" json:"Input,omitempty"`
}

type TransactionsVolume struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Volume        float64            `bson:"volume"`
	TransferCount int                `bson:"transferCount"`
	Timestamp     string             `bson:"timestamp"`
}
