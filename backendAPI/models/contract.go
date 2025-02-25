package models

type ContractInfo struct {
	ContractCreatorAddress string `json:"contractCreatorAddress" bson:"contractCreatorAddress"`
	ContractAddress        string `json:"contractAddress" bson:"contractAddress"`
	ContractCode           string `json:"contractCode" bson:"contractCode"`
	CreationTransaction    string `json:"creationTransaction" bson:"creationTransaction"`
	IsToken                bool   `json:"isToken" bson:"isToken"`
	Status                 string `json:"status" bson:"status"`
	TokenDecimals          uint8  `json:"tokenDecimals" bson:"tokenDecimals"`
	TokenName              string `json:"tokenName" bson:"tokenName"`
	TokenSymbol            string `json:"tokenSymbol" bson:"tokenSymbol"`
	UpdatedAt              string `json:"updatedAt" bson:"updatedAt"`
}
