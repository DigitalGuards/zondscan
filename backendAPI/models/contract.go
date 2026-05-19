package models

// ContractInfo is the persisted contract document in the `contractCode`
// MongoDB collection.
//
// The verification fields at the bottom are written **only** by the backend
// verify endpoint (see backendAPI/db/contract.go:MarkContractVerified). The
// syncer (QRL2MongoDB) treats them as opaque pass-through state and must
// never write into them — see QRL2MongoDB/db/contracts.go:StoreContract for
// the field-scoped $set that enforces this invariant.
//
// New verification fields MUST be mirrored in QRL2MongoDB/models/contract.go
// with identical bson and json tags so the syncer's whole-doc reads survive
// round-trips.
type ContractInfo struct {
	ContractCreatorAddress string `json:"creatorAddress" bson:"creatorAddress"`
	ContractAddress        string `json:"address" bson:"address"`
	ContractCode           string `json:"contractCode" bson:"contractCode"`
	CreationTransaction    string `json:"creationTransaction" bson:"creationTransaction"`
	CreationBlockNumber    string `json:"creationBlockNumber" bson:"creationBlockNumber"`
	IsToken                bool   `json:"isToken" bson:"isToken"`
	Status                 string `json:"status" bson:"status"`
	TokenDecimals          uint8  `json:"decimals" bson:"decimals"`
	TokenName              string `json:"name" bson:"name"`
	TokenSymbol            string `json:"symbol" bson:"symbol"`
	TotalSupply            string `json:"totalSupply" bson:"totalSupply"`
	UpdatedAt              string `json:"updatedAt" bson:"updatedAt"`

	// Source-verification fields. `verified` defaults to false (omitted from
	// omitempty so it is always present in the JSON shape clients consume).
	// Everything else uses omitempty so unverified contracts stay clean.
	Verified             bool              `json:"verified" bson:"verified"`
	SourceCode           string            `json:"sourceCode,omitempty" bson:"sourceCode,omitempty"`
	Abi                  string            `json:"abi,omitempty" bson:"abi,omitempty"`
	ContractName         string            `json:"contractName,omitempty" bson:"contractName,omitempty"`
	CompilerVersion      string            `json:"compilerVersion,omitempty" bson:"compilerVersion,omitempty"`
	OptimizationEnabled  bool              `json:"optimizationEnabled" bson:"optimizationEnabled"`
	OptimizationRuns     int               `json:"optimizationRuns" bson:"optimizationRuns"`
	EvmVersion           string            `json:"evmVersion,omitempty" bson:"evmVersion,omitempty"`
	ConstructorArguments string            `json:"constructorArguments,omitempty" bson:"constructorArguments,omitempty"`
	Libraries            map[string]string `json:"libraries,omitempty" bson:"libraries,omitempty"`
	License              string            `json:"license,omitempty" bson:"license,omitempty"`
	VerificationMethod   string            `json:"verificationMethod,omitempty" bson:"verificationMethod,omitempty"`
	VerifiedAt           string            `json:"verifiedAt,omitempty" bson:"verifiedAt,omitempty"`
}
