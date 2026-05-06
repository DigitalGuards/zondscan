package models

type TransactionData struct {
	BlockHash           string `json:"blockHash" bson:"blockHash"`
	BlockNumber         uint64 `json:"blockNumber" bson:"blockNumber"`
	From                string `json:"from" bson:"from"`
	Hash                string `json:"hash" bson:"hash"`
	Nonce               uint64 `json:"nonce" bson:"nonce"`
	TransactionIndex    uint64 `json:"transactionIndex" bson:"transactionIndex"`
	BlockProposerReward uint64 `json:"blockProposerReward" bson:"blockProposerReward"`
	AttestorReward      uint64 `json:"attestorReward" bson:"attestorReward"`
	FeeReward           uint64 `json:"feeReward" bson:"feeReward"`
	TxType              uint8  `json:"txType" bson:"txType"`
	ChainId             uint8  `json:"chainId" bson:"chainId"`
	Signature           string `json:"signature" bson:"signature"`
	PublicKey           string `json:"publicKey" bson:"publicKey"`
}
