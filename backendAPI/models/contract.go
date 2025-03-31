package models

type ContractInfo struct {
	ContractCreatorAddress string `json:"creatorAddress" bson:"creatorAddress"`
	ContractAddress        string `json:"address" bson:"address"`
	ContractCode           string `json:"contractCode" bson:"contractCode"`
	CreationTransaction    string `json:"creationTransaction" bson:"creationTransaction"`
	IsToken                bool   `json:"isToken" bson:"isToken"`
	Status                 string `json:"status" bson:"status"`
	TokenDecimals          uint8  `json:"decimals" bson:"decimals"`
	TokenName              string `json:"name" bson:"name"`
	TokenSymbol            string `json:"symbol" bson:"symbol"`
	UpdatedAt              string `json:"updatedAt" bson:"updatedAt"`
}
