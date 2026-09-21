package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Transfer struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	BlockNumber       string             `bson:"blockNumber"`
	BlockHash         string             `bson:"blockHash"`
	BlockTimestamp    string             `bson:"blockTimestamp"`
	From              string             `bson:"from"`
	To                string             `bson:"to"`
	TxHash            string             `bson:"txHash"`
	Value             string             `bson:"value"`
	GasUsed           string             `bson:"gasUsed"`
	GasLimit          string             `bson:"gasLimit"`
	GasPrice          string             `bson:"gasPrice"`
	EffectiveGasPrice string             `bson:"effectiveGasPrice"`
	Status            string             `bson:"status"`
	PaidFees          string             `bson:"-" json:"PaidFees,omitempty"`
	Nonce             string             `bson:"nonce"`
	Signature         string             `bson:"signature"`
	Pk                string             `bson:"pk"`
	Size              string             `bson:"size"`
	// Input is RPC calldata retained under the compatible BSON data field.
	// "0x" identifies known empty input; an empty string is unavailable legacy
	// input. Drives the Input Data card and contract-call decoding.
	Input string `bson:"input,omitempty" json:"Input,omitempty"`
}

type TransactionsVolume struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Volume        float64            `bson:"volume"`
	TransferCount int                `bson:"transferCount"`
	Timestamp     string             `bson:"timestamp"`
}
