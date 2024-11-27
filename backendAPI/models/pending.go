package models

// PendingTransaction represents a single pending transaction
type PendingTransaction struct {
	AccessList           []interface{} `json:"accessList"`
	BlockHash            interface{}   `json:"blockHash"`
	BlockNumber          interface{}   `json:"blockNumber"`
	ChainId              string        `json:"chainId"`
	From                 string        `json:"from"`
	Gas                  string        `json:"gas"`
	GasPrice             string        `json:"gasPrice"`
	Hash                 string        `json:"hash"`
	Input                string        `json:"input"`
	MaxFeePerGas         string        `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string        `json:"maxPriorityFeePerGas"`
	Nonce                string        `json:"nonce"`
	PublicKey            string        `json:"publicKey"`
	To                   string        `json:"to"`
	TransactionIndex     interface{}   `json:"transactionIndex"`
	Type                 string        `json:"type"`
	Value                string        `json:"value"`
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
