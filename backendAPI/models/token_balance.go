package models

// TokenBalance represents a token holding for a specific address
type TokenBalance struct {
	ContractAddress string `json:"contractAddress" bson:"contractAddress"`
	HolderAddress   string `json:"holderAddress" bson:"holderAddress"`
	Balance         string `json:"balance" bson:"balance"`
	BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
	UpdatedAt       string `json:"updatedAt" bson:"updatedAt"`
	// Token metadata (populated via aggregation)
	Name     string `json:"name,omitempty" bson:"name,omitempty"`
	Symbol   string `json:"symbol,omitempty" bson:"symbol,omitempty"`
	Decimals int    `json:"decimals,omitempty" bson:"decimals,omitempty"`
}

// TokenBalancesResponse is the API response for token balances
type TokenBalancesResponse struct {
	Address string         `json:"address"`
	Tokens  []TokenBalance `json:"tokens"`
	Count   int            `json:"count"`
}

// TokenTransfer represents a token transfer event
type TokenTransfer struct {
	ContractAddress string `json:"contractAddress" bson:"contractAddress"`
	From            string `json:"from" bson:"from"`
	To              string `json:"to" bson:"to"`
	Amount          string `json:"amount" bson:"amount"`
	BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
	TxHash          string `json:"txHash" bson:"txHash"`
	Timestamp       string `json:"timestamp" bson:"timestamp"`
	TokenSymbol     string `json:"tokenSymbol" bson:"tokenSymbol"`
	TokenDecimals   int    `json:"tokenDecimals" bson:"tokenDecimals"`
	TokenName       string `json:"tokenName" bson:"tokenName"`
	TransferType    string `json:"transferType" bson:"transferType"`
}

// TokenHoldersResponse is the API response for token holders
type TokenHoldersResponse struct {
	ContractAddress string         `json:"contractAddress"`
	Holders         []TokenBalance `json:"holders"`
	TotalHolders    int            `json:"totalHolders"`
	Page            int            `json:"page"`
	Limit           int            `json:"limit"`
}

// TokenTransfersResponse is the API response for token transfers
type TokenTransfersResponse struct {
	ContractAddress string          `json:"contractAddress"`
	Transfers       []TokenTransfer `json:"transfers"`
	TotalTransfers  int64           `json:"totalTransfers"`
	Page            int             `json:"page"`
	Limit           int             `json:"limit"`
}

// TokenInfo contains summary information about a token
type TokenInfo struct {
	ContractAddress string `json:"contractAddress"`
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Decimals        int    `json:"decimals"`
	TotalSupply     string `json:"totalSupply"`
	HolderCount     int    `json:"holderCount"`
	TransferCount   int64  `json:"transferCount"`
	CreatorAddress  string `json:"creatorAddress"`
	CreationTxHash  string `json:"creationTxHash"`
	CreationBlock   string `json:"creationBlock"`
}
