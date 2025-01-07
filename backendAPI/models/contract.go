package models

type ContractCode struct {
	ContractCreatorAddress []byte `json:"contractCreatorAddress" bson:"contractCreatorAddress"`
	ContractAddress        []byte `json:"contractAddress" bson:"contractAddress"`
	ContractCode           []byte `json:"contractCode" bson:"contractCode"`
	// Token information
	TokenName     string `json:"tokenName,omitempty" bson:"tokenName,omitempty"`
	TokenSymbol   string `json:"tokenSymbol,omitempty" bson:"tokenSymbol,omitempty"`
	TokenDecimals uint8  `json:"tokenDecimals,omitempty" bson:"tokenDecimals,omitempty"`
	IsToken       bool   `json:"isToken" bson:"isToken"`
}
