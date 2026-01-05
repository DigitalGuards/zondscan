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
