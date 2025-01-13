package models

import (
	"time"
)

// PendingTransaction represents a transaction in the mempool
type PendingTransaction struct {
	Hash                 string    `bson:"_id" json:"hash"`
	From                 string    `bson:"from" json:"from"`
	To                   string    `bson:"to" json:"to"`
	Value               string    `bson:"value" json:"value"`
	Gas                 string    `bson:"gas" json:"gas"`
	GasPrice            string    `bson:"gasPrice" json:"gasPrice"`
	Input               string    `bson:"input" json:"input"`
	Nonce               string    `bson:"nonce" json:"nonce"`
	Type                string    `bson:"type" json:"type"`
	ChainId             string    `bson:"chainId" json:"chainId"`
	LastSeen            time.Time `bson:"lastSeen" json:"lastSeen"`
	Status              string    `bson:"status" json:"status"` // "pending", "mined", "dropped"
	CreatedAt           time.Time `bson:"createdAt" json:"createdAt"`
}

// PendingTransactionResponse represents the RPC response for pending transactions
type PendingTransactionResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  struct {
		Pending map[string]map[string]PendingTransaction `json:"pending"`
		Queued  map[string]map[string]PendingTransaction `json:"queued"`
	} `json:"result"`
}
