package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ZondDatabaseBlock struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  Result `json:"result"`
}

type Withdrawal struct {
	Index          string `json:"index"`
	ValidatorIndex string `json:"validatorIndex"`
	Address        string `json:"address"`
	Amount         string `json:"amount"`
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

type Result struct {
	BaseFeePerGas    string        `json:"baseFeePerGas"`
	GasLimit         string        `json:"gasLimit"`
	GasUsed          string        `json:"gasUsed"`
	Hash             string        `json:"hash"`
	Number           string        `json:"number"`
	ParentHash       string        `json:"parentHash"`
	ReceiptsRoot     string        `json:"receiptsRoot"`
	StateRoot        string        `json:"stateRoot"`
	Timestamp        string        `json:"timestamp"`
	Transactions     []Transaction `json:"transactions"`
	TransactionsRoot string        `json:"transactionsRoot"`
	ExtraData        string        `json:"extraData"`
	LogsBloom        string        `json:"logsBloom"`
	Miner            string        `json:"miner"`
	Size             string        `json:"size"`
	PrevRandao       string        `json:"prevRandao"`
	Withdrawals      []Withdrawal  `json:"withdrawals"`
	WithdrawalsRoot  string        `json:"withdrawalsRoot"`
}

type CirculatingSupply struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Circulating string             `bson:"circulating"`
}
