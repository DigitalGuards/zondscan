package models

type ContractCode struct {
	ContractCreatorAddress []byte `json:"contractCreatorAddress"`
	ContractAddress        []byte `json:"contractAddress"`
	ContractCode           []byte `json:"contractCode"`
}
