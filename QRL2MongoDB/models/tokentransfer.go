package models

// TokenTransfer represents a token transfer event
type TokenTransfer struct {
	ContractAddress string `bson:"contractAddress"`
	From            string `bson:"from"`
	To              string `bson:"to"`
	Amount          string `bson:"amount"`
	BlockNumber     string `bson:"blockNumber"`
	TxHash          string `bson:"txHash"`
	Timestamp       string `bson:"timestamp"`
	TokenSymbol     string `bson:"tokenSymbol"`
	TokenDecimals   uint8  `bson:"tokenDecimals"`
	TokenName       string `bson:"tokenName"`
	TransferType    string `bson:"transferType"` // "direct" for direct transfers, "event" for Transfer events
}
