package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Transfer struct {
	ID             primitive.ObjectID `bson:"_id"`
	BlockNumber    uint64             `json:"blockNumber"`
	BlockTimestamp uint64             `json:"blockTimestamp"`
	From           []byte             `json:"from"`
	To             []byte             `json:"to"`
	TxHash         []byte             `json:"txHash"`
	Pk             []byte             `json:"pk"`
	Signature      []byte             `json:"signature"`
	Nonce          uint64             `json:"nonce"`
	Value          uint64             `json:"value"`
	ValueStr       string             `json:"valueStr,omitempty"` // Added for hex string representation
	GasUsed        uint64             `json:"gasUsed"`
	GasPrice       uint64             `json:"gasPrice"`
	GasUsedStr     string             `json:"gasUsedStr,omitempty"`  // Added for hex string representation
	GasPriceStr    string             `json:"gasPriceStr,omitempty"` // Added for hex string representation
	Status         uint32             `json:"status"`
	Size           uint64             `json:"size"`
	FromStr        string             `json:"fromStr,omitempty"` // Added for hex string representation
	ToStr          string             `json:"toStr,omitempty"`   // Added for hex string representation
}

type TransactionsVolume struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Volume int64              `bson:"volume"`
}
