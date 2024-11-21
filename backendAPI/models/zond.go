package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/big"
	)

type Withdrawal struct {
	Index          string   `json:"index"`
	ValidatorIndex string   `json:"validatorIndex"`
	Address        string   `json:"address"`
	Amount         *big.Int `json:"amount"`
}

type Transaction struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Type             string `json:"type"`
	Value            string `json:"value"`
	ChainID          string `json:"chainId"`
	Signature        string `json:"signature"`
	PublicKey        string `json:"publicKey"`
	Data             string `json:"data"`
	Status           string `json:"status"`
}

type ProtocolTransactions struct {
	BlockHash           string `json:"blockHash"`
	BlockNumber         string `json:"blockNumber"`
	From                string `json:"from"`
	Hash                string `json:"hash"`
	Nonce               string `json:"nonce"`
	TransactionIndex    string `json:"transactionIndex"`
	BlockProposerReward string `json:"blockProposerReward,omitempty"`
	AttestorReward      string `json:"attestorReward,omitempty"`
	FeeReward           string `json:"feeReward,omitempty"`
	Type                string `json:"type"`
	ChainID             string `json:"chainId"`
	Signature           string `json:"signature"`
	Pk                  string `json:"pk"`
}
type Transactions struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Type             string `json:"type"`
	Value            string `json:"value"`
	ChainID          string `json:"chainId"`
	Signature        string `json:"signature"`
	Pk               string `json:"pk"`
	Data             string `json:"data"`
	Status           string `json:"status"`
}

type ResultOld struct {
    BaseFeePerGas        string                `json:"baseFeePerGas"`
    GasLimit             string                `json:"gasLimit"`
    GasUsed              string                `json:"gasUsed"`
    Hash                 string                `json:"hash"`
    Number               string                `json:"number"`
    ParentHash           string                `json:"parentHash"`
    ReceiptsRoot         string                `json:"receiptsRoot"`
    StateRoot            string                `json:"stateRoot"`
    Timestamp            string                `json:"timestamp"`
    Transactions         []Transaction         `json:"transactions"`
    TransactionsRoot     string                `json:"transactionsRoot"`
    Difficulty           string                `json:"difficulty"`
    ExtraData            string                `json:"extraData"`
    LogsBloom            string                `json:"logsBloom"`
    Miner                string                `json:"miner"`
    MixHash              string                `json:"mixHash"`
    Nonce                string                `json:"nonce"`
    Sha3Uncles           string                `json:"sha3Uncles"`
    Size                 string                `json:"size"`
    TotalDifficulty      string                `json:"totalDifficulty"`
    Uncles               []interface{}         `json:"uncles"`
    Withdrawals          []Withdrawal          `json:"withdrawals"`
    WithdrawalsRoot      string                `json:"withdrawalsRoot"`
}

type Result struct {
    BaseFeePerGas        uint64                `json:"baseFeePerGas"`
    GasLimit             uint64                `json:"gasLimit"`
    GasUsed              uint64                `json:"gasUsed"`
    Hash                 string                `json:"hash"`
    Number               uint64                `json:"number"`
    ParentHash           string                `json:"parentHash"`
    ReceiptsRoot         string                `json:"receiptsRoot"`
    StateRoot            string                `json:"stateRoot"`
    Timestamp            uint64                `json:"timestamp"`
    Transactions         []Transaction         `json:"transactions"`
    TransactionsRoot     string                `json:"transactionsRoot"`
    Difficulty           uint64                `json:"difficulty"`
    ExtraData            string                `json:"extraData"`
    LogsBloom            string                `json:"logsBloom"`
    Miner                string                `json:"miner"`
    MixHash              string                `json:"mixHash"`
    Nonce                string                `json:"nonce"`
    Sha3Uncles           string                `json:"sha3Uncles"`
    Size                 uint64                `json:"size"`
    TotalDifficulty      uint64                `json:"totalDifficulty"`
    Uncles               []interface{}         `json:"uncles"`
    Withdrawals          []Withdrawal          `json:"withdrawals"`
    WithdrawalsRoot      string                `json:"withdrawalsRoot"`
}

type CirculatingSupply struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Circulating string             `bson:"circulating"`
}
