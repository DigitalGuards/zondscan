package models

// TransactionResponse represents the RPC response for a transaction lookup
type TransactionResponse struct {
	Jsonrpc string            `json:"jsonrpc"`
	ID      int               `json:"id"`
	Result  TransactionResult `json:"result"`
}

// TransactionResult represents the detailed transaction data returned by the RPC
type TransactionResult struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
	Type             string `json:"type"`
	ChainId          string `json:"chainId"`
	V                string `json:"v"`
	R                string `json:"r"`
	S                string `json:"s"`
}
