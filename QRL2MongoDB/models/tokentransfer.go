package models

// TokenTransfer represents a token transfer event.
//
// LogIndex disambiguates multiple Transfer events within the same tx (a
// DEX swap typically emits 3 — router → pool → user). Without it the
// `tokenTransfers` collection used to enforce a UNIQUE index on txHash
// alone, so every event after the first in a given tx silently failed
// to persist (the InsertOne returned a duplicate-key error and balance
// updates short-circuited). Stored as the hex string the RPC returns
// (e.g. "0x0", "0x1", "0x2"). The legacy direct-calldata path that
// pre-dates this fix writes `LogIndex` empty + `TransferType == "direct"`
// to remain distinct from any event-derived row for the same call.
type TokenTransfer struct {
	ContractAddress string `bson:"contractAddress"`
	From            string `bson:"from"`
	To              string `bson:"to"`
	Amount          string `bson:"amount"`
	BlockNumber     string `bson:"blockNumber"`
	TxHash          string `bson:"txHash"`
	LogIndex        string `bson:"logIndex,omitempty"`
	Timestamp       string `bson:"timestamp"`
	TokenSymbol     string `bson:"tokenSymbol"`
	TokenDecimals   uint8  `bson:"tokenDecimals"`
	TokenName       string `bson:"tokenName"`
	TransferType    string `bson:"transferType"` // "direct" for direct transfers, "event" for Transfer events
}
