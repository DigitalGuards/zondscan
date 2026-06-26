package models

// TokenTransfer represents a token transfer event.
//
// LogIndex disambiguates multiple Transfer events within the same tx (a
// DEX swap typically emits 3, router → pool → user). Without it the
// `tokenTransfers` collection used to enforce a UNIQUE index on txHash
// alone, so every event after the first in a given tx silently failed
// to persist (the InsertOne returned a duplicate-key error and balance
// updates short-circuited). Stored as the hex string the RPC returns
// (e.g. "0x0", "0x1", "0x2"). The legacy direct-calldata path that
// pre-dates this fix writes `LogIndex` empty + `TransferType == "direct"`
// to remain distinct from any event-derived row for the same call.
type TokenTransfer struct {
	ContractAddress string `bson:"contractAddress"`
	From            string `bson:"from"`
	To              string `bson:"to"`
	Amount          string `bson:"amount"`
	BlockNumber     string `bson:"blockNumber"`
	// BlockNumberInt is the numeric form of BlockNumber, set at write time
	// via HexToInt64. Sorting on the hex string lex-orders incorrectly
	// ("0x9" sorts after "0x10", and width boundaries like 0xffff -> 0x10000
	// break ordering entirely), so all sort operations use this field instead.
	BlockNumberInt int64  `bson:"blockNumberInt"`
	TxHash         string `bson:"txHash"`
	// Stored ALWAYS, including the empty-string sentinel for the
	// direct-calldata path. The `omitempty` tag would drop the field
	// when LogIndex is "", and then BSON queries `{logIndex: ""}`
	// would no longer match those documents, which is exactly what
	// TokenTransferExists relies on for idempotent reprocess.
	LogIndex      string `bson:"logIndex"`
	Timestamp     string `bson:"timestamp"`
	TokenSymbol   string `bson:"tokenSymbol"`
	TokenDecimals uint8  `bson:"tokenDecimals"`
	TokenName     string `bson:"tokenName"`
	TransferType  string `bson:"transferType"` // "direct" for direct transfers, "event" for Transfer events
	// TokenStandard denormalises ContractInfo.TokenStandard for query
	// convenience (so /token-transfer endpoints can filter by standard
	// without joining contractCode). Empty for legacy ERC-20 rows.
	TokenStandard string `bson:"tokenStandard,omitempty" json:"tokenStandard,omitempty"`
	// TokenID is the per-event uint256 token identifier (decimal string).
	// Empty for ERC-20; populated for ERC-721 (topic[3]) and ERC-1155
	// (data field). One row per (id, value) tuple in TransferBatch logs,
	// so the compound unique index (txHash, contract, logIndex, tokenID)
	// keeps batch elements distinct.
	TokenID string `bson:"tokenID,omitempty" json:"tokenID,omitempty"`
}
