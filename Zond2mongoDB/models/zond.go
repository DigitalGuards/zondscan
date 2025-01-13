package models

import (
	"math/big"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RPC struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  string `json:"result"`
}

type Zond struct {
	Jsonrpc   string    `json:"jsonrpc"`
	ID        int       `json:"id"`
	PreResult PreResult `json:"result"`
}

type ZondDatabaseBlock struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  Result `json:"result"`
}

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

type PreResult struct {
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
	Difficulty       string        `json:"difficulty"`
	ExtraData        string        `json:"extraData"`
	LogsBloom        string        `json:"logsBloom"`
	Miner            string        `json:"miner"`
	MixHash          string        `json:"mixHash"`
	Nonce            string        `json:"nonce"`
	Sha3Uncles       string        `json:"sha3Uncles"`
	Size             string        `json:"size"`
	TotalDifficulty  string        `json:"totalDifficulty"`
	Uncles           []interface{} `json:"uncles"`
	Withdrawals      []Withdrawal  `json:"withdrawals"`
	WithdrawalsRoot  string        `json:"withdrawalsRoot"`
}

type Result struct {
	BaseFeePerGas    uint64        `json:"baseFeePerGas"`
	GasLimit         uint64        `json:"gasLimit"`
	GasUsed          uint64        `json:"gasUsed"`
	Hash             string        `json:"hash"`
	Number           uint64        `json:"number"`
	ParentHash       string        `json:"parentHash"`
	ReceiptsRoot     string        `json:"receiptsRoot"`
	StateRoot        string        `json:"stateRoot"`
	Timestamp        uint64        `json:"timestamp"`
	Transactions     []Transaction `json:"transactions"`
	TransactionsRoot string        `json:"transactionsRoot"`
	Difficulty       uint64        `json:"difficulty"`
	ExtraData        string        `json:"extraData"`
	LogsBloom        string        `json:"logsBloom"`
	Miner            string        `json:"miner"`
	MixHash          string        `json:"mixHash"`
	Nonce            string        `json:"nonce"`
	Sha3Uncles       string        `json:"sha3Uncles"`
	Size             uint64        `json:"size"`
	TotalDifficulty  uint64        `json:"totalDifficulty"`
	Uncles           []interface{} `json:"uncles"`
	Withdrawals      []Withdrawal  `json:"withdrawals"`
	WithdrawalsRoot  string        `json:"withdrawalsRoot"`
}

type JsonRPC struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type GetBalance struct {
	JsonRPC string `json:"jsonrpc"`
	ID      uint32 `json:"id"`
	Result  string `json:"result"`
}

type GetCode struct {
	JsonRPC string `json:"jsonrpc"`
	ID      uint32 `json:"id"`
	Result  string `json:"result"`
}

type ZondCallPayload struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type ZondResponse struct {
	Id      int        `json:"id"`
	Jsonrpc string     `json:"jsonrpc"`
	Result  ZondResult `json:"result"`
}

type ZondResult struct {
}

type Vote struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Option string             `bson:"option"`
	Count  *big.Int           `bson:"count"`
}
