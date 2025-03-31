package models

type Contract struct {
	Jsonrpc string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  ResultContract `json:"result"`
}

type Logs struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

type ResultContract struct {
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	EffectiveGasPrice string `json:"effectiveGasPrice"`
	From              string `json:"from"`
	GasUsed           string `json:"gasUsed"`
	Logs              []Logs `json:"logs"`
	LogsBloom         string `json:"logsBloom"`
	Status            string `json:"status"`
	To                string `json:"to"`
	TransactionHash   string `json:"transactionHash"`
	TransactionIndex  string `json:"transactionIndex"`
	Type              string `json:"type"`
}

// ContractInfo represents contract information stored in MongoDB
type ContractInfo struct {
	Address             string `bson:"address" json:"address"`
	Status              string `bson:"status" json:"status"`
	IsToken             bool   `bson:"isToken" json:"isToken"`
	Name                string `bson:"name" json:"name"`
	Symbol              string `bson:"symbol" json:"symbol"`
	Decimals            uint8  `bson:"decimals" json:"decimals"`
	TotalSupply         string `bson:"totalSupply" json:"totalSupply"`
	ContractCode        string `bson:"contractCode" json:"contractCode"`
	CreatorAddress      string `bson:"creatorAddress" json:"creatorAddress"`
	CreationTransaction string `bson:"creationTransaction" json:"creationTransaction"`
	CreationBlockNumber string `bson:"creationBlockNumber" json:"creationBlockNumber"`
	UpdatedAt           string `bson:"updatedAt" json:"updatedAt"`
	// CustomERC20 properties
	MaxSupply       string `bson:"maxSupply,omitempty" json:"maxSupply,omitempty"`
	MaxWalletAmount string `bson:"maxWalletAmount,omitempty" json:"maxWalletAmount,omitempty"`
	MaxTxLimit      string `bson:"maxTxLimit,omitempty" json:"maxTxLimit,omitempty"`
}

// LogsResponse represents the response from zond_getLogs
type LogsResponse struct {
	Jsonrpc string     `json:"jsonrpc"`
	ID      int        `json:"id"`
	Result  []LogEntry `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// LogEntry represents a single log entry from zond_getLogs
type LogEntry struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}
