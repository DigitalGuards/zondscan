package models

import (
	"encoding/json"
	"time"
)

// PendingTransaction represents a single pending transaction
type PendingTransaction struct {
	AccessList           []interface{} `json:"accessList" bson:"accessList"`
	BlockHash            interface{}   `json:"blockHash" bson:"blockHash"`
	BlockNumber          interface{}   `json:"blockNumber" bson:"blockNumber"`
	ChainId              string        `json:"chainId" bson:"chainId"`
	From                 string        `json:"from" bson:"from"`
	Gas                  string        `json:"gas" bson:"gas"`
	GasPrice             string        `json:"gasPrice" bson:"gasPrice"`
	Hash                 string        `json:"hash" bson:"_id"`
	Input                string        `json:"input" bson:"input"`
	MaxFeePerGas         string        `json:"maxFeePerGas,omitempty" bson:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas string        `json:"maxPriorityFeePerGas,omitempty" bson:"maxPriorityFeePerGas,omitempty"`
	Nonce                string        `json:"nonce" bson:"nonce"`
	PublicKey            string        `json:"publicKey" bson:"publicKey"`
	To                   string        `json:"to,omitempty" bson:"to,omitempty"`
	TransactionIndex     interface{}   `json:"transactionIndex" bson:"transactionIndex"`
	Type                 string        `json:"type" bson:"type"`
	Value                string        `json:"value" bson:"value"`
	Status               string        `json:"status" bson:"status"` // "pending", "mined", "dropped"
	LastSeen             time.Time     `json:"lastSeen" bson:"lastSeen"`
	CreatedAt            time.Time     `json:"createdAt" bson:"createdAt"`
}

// MarshalJSON customizes JSON marshaling to convert timestamps to Unix timestamps
func (p *PendingTransaction) MarshalJSON() ([]byte, error) {
	type Alias PendingTransaction
	return json.Marshal(&struct {
		LastSeen  int64 `json:"lastSeen"`
		CreatedAt int64 `json:"createdAt"`
		*Alias
	}{
		LastSeen:  p.LastSeen.Unix(),
		CreatedAt: p.CreatedAt.Unix(),
		Alias:     (*Alias)(p),
	})
}

// PendingTransactionsResponse represents the response from txpool_content RPC call
type PendingTransactionsResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  struct {
		Pending map[string]map[string]PendingTransaction `json:"pending"`
		Queued  map[string]map[string]PendingTransaction `json:"queued"`
	} `json:"result"`
}

type PaginatedPendingTransactions struct {
	Transactions []PendingTransaction `json:"transactions"`
	Total       int                  `json:"total"`
	Page        int                  `json:"page"`
	Limit       int                  `json:"limit"`
	TotalPages  int                  `json:"totalPages"`
}

type PaginatedPendingTransactionsResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  PaginatedPendingTransactions `json:"result"`
}

type PendingTransactionResponse struct {
	Transaction *PendingTransaction `json:"transaction"`
}
