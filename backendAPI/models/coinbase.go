package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Coinbase struct {
	ID                  primitive.ObjectID `bson:"_id"`
	BlockHash           []byte             `json:"blockHash"`
	BlockNumber         uint64             `json:"blockNumber"`
	From                []byte             `json:"from"`
	Hash                []byte             `json:"hash"`
	Nonce               uint64             `json:"nonce"`
	TransactionIndex    uint64             `json:"transactionindex"`
	Blockproposerreward uint64             `json:"blockproposerreward"`
	AttestorReward      uint64             `json:"attestorReward"`
	FeeReward           uint64             `json:"feeReward"`
	Type                uint64             `json:"type"`
	ChainId             uint64             `json:"chainid"`
	Signature           []byte             `json:"signature"`
	Pk                  []byte             `json:"Pk"`
}
