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

// ContractInfo stores contract metadata including token information
type ContractInfo struct {
	ContractCreatorAddress []byte `json:"contractCreatorAddress" bson:"contractCreatorAddress"`
	ContractAddress        []byte `json:"contractAddress" bson:"contractAddress"`
	ContractCode           []byte `json:"contractCode" bson:"contractCode"`
	// Token information
	TokenName     string `json:"tokenName,omitempty" bson:"tokenName,omitempty"`
	TokenSymbol   string `json:"tokenSymbol,omitempty" bson:"tokenSymbol,omitempty"`
	TokenDecimals uint8  `json:"tokenDecimals,omitempty" bson:"tokenDecimals,omitempty"`
	IsToken       bool   `json:"isToken" bson:"isToken"`
}
