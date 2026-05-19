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

// ContractInfo represents contract information stored in MongoDB.
//
// The verification fields at the bottom are written **only** by the
// backendAPI verify endpoint (backendAPI/db/contract.go:MarkContractVerified).
// The syncer treats them as opaque pass-through state and must never write
// into them — see db/contracts.go:StoreContract for the field-scoped $set
// that enforces this invariant.
//
// Verification fields are mirrored in backendAPI/models/contract.go with
// identical bson/json tags so whole-doc reads survive cross-process
// round-trips.
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

	// Source-verification fields — mirror backendAPI/models/contract.go.
	// Written exclusively by the backend verify endpoint; the syncer
	// holds them only for round-trip preservation.
	Verified             bool              `bson:"verified" json:"verified"`
	SourceCode           string            `bson:"sourceCode,omitempty" json:"sourceCode,omitempty"`
	Abi                  string            `bson:"abi,omitempty" json:"abi,omitempty"`
	ContractName         string            `bson:"contractName,omitempty" json:"contractName,omitempty"`
	CompilerVersion      string            `bson:"compilerVersion,omitempty" json:"compilerVersion,omitempty"`
	OptimizationEnabled  bool              `bson:"optimizationEnabled" json:"optimizationEnabled"`
	OptimizationRuns     int               `bson:"optimizationRuns" json:"optimizationRuns"`
	EvmVersion           string            `bson:"evmVersion,omitempty" json:"evmVersion,omitempty"`
	ConstructorArguments string            `bson:"constructorArguments,omitempty" json:"constructorArguments,omitempty"`
	Libraries            map[string]string `bson:"libraries,omitempty" json:"libraries,omitempty"`
	License              string            `bson:"license,omitempty" json:"license,omitempty"`
	VerificationMethod   string            `bson:"verificationMethod,omitempty" json:"verificationMethod,omitempty"`
	VerifiedAt           string            `bson:"verifiedAt,omitempty" json:"verifiedAt,omitempty"`

	// M6a AI explanation cache. Written exclusively by the backend
	// /contract/explain endpoint; the syncer holds them only for
	// round-trip preservation.
	AIExplanation      string `bson:"aiExplanation,omitempty" json:"aiExplanation,omitempty"`
	AIExplanationAt    string `bson:"aiExplanationAt,omitempty" json:"aiExplanationAt,omitempty"`
	AIExplanationModel string `bson:"aiExplanationModel,omitempty" json:"aiExplanationModel,omitempty"`

	AIExplanationRegenCount       int    `bson:"aiExplanationRegenCount,omitempty" json:"aiExplanationRegenCount,omitempty"`
	AIExplanationRegenWindowStart string `bson:"aiExplanationRegenWindowStart,omitempty" json:"aiExplanationRegenWindowStart,omitempty"`
}

// LogsResponse represents the response from qrl_getLogs
type LogsResponse struct {
	Jsonrpc string     `json:"jsonrpc"`
	ID      int        `json:"id"`
	Result  []LogEntry `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// LogEntry represents a single log entry from qrl_getLogs
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
