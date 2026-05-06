package models

// TransactionReceipt represents a transaction receipt with logs
type TransactionReceipt struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		BlockHash         string `json:"blockHash"`
		BlockNumber      string `json:"blockNumber"`
		ContractAddress  string `json:"contractAddress"`
		CumulativeGasUsed string `json:"cumulativeGasUsed"`
		From             string `json:"from"`
		GasUsed         string `json:"gasUsed"`
		Logs            []Log  `json:"logs"`
		LogsBloom       string `json:"logsBloom"`
		Status          string `json:"status"`
		To              string `json:"to"`
		TransactionHash string `json:"transactionHash"`
		TransactionIndex string `json:"transactionIndex"`
	} `json:"result"`
}

// Log represents a log entry in a transaction receipt
type Log struct {
	Address          string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	TransactionHash string   `json:"transactionHash"`
	TransactionIndex string  `json:"transactionIndex"`
	BlockHash       string   `json:"blockHash"`
	LogIndex        string   `json:"logIndex"`
	Removed         bool     `json:"removed"`
}
