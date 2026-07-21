package models

// TokenBalance represents a token holding for a specific address.
//
// Phase 2: NFT collections store one row per (contract, holder, tokenID) so
// the same struct serves ERC-20 (TokenID empty), ERC-721 (Balance="1" per id),
// and ERC-1155 (Balance is per-id quantity as a decimal string). Legacy
// pre-Phase-2 rows have no TokenID / TokenStandard fields, the omitempty
// tags keep their JSON shape stable.
type TokenBalance struct {
	ContractAddress string `json:"contractAddress" bson:"contractAddress"`
	HolderAddress   string `json:"holderAddress" bson:"holderAddress"`
	Balance         string `json:"balance" bson:"balance"`
	BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
	UpdatedAt       string `json:"updatedAt" bson:"updatedAt"`
	TokenID         string `json:"tokenID,omitempty" bson:"tokenID,omitempty"`
	TokenStandard   string `json:"tokenStandard,omitempty" bson:"tokenStandard,omitempty"`
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

// TokenTransfer represents a token transfer event.
//
// TokenStandard / TokenID / LogIndex mirror the syncer's struct
// (QRL2MongoDB/models/tokentransfer.go). Without them the BSON decoder
// silently strips the fields off the persisted document and the tx page
// can't tell an NFT transfer from a fungible one. LogIndex is required
// to disambiguate multiple events in the same tx (DEX swaps, ERC-1155
// TransferBatch fan-out).
type TokenTransfer struct {
	ContractAddress string `json:"contractAddress" bson:"contractAddress"`
	From            string `json:"from" bson:"from"`
	To              string `json:"to" bson:"to"`
	Amount          string `json:"amount" bson:"amount"`
	BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
	BlockNumberInt  int64  `json:"blockNumberInt" bson:"blockNumberInt"`
	TxHash          string `json:"txHash" bson:"txHash"`
	LogIndex        string `json:"logIndex,omitempty" bson:"logIndex,omitempty"`
	Timestamp       string `json:"timestamp" bson:"timestamp"`
	TokenSymbol     string `json:"tokenSymbol" bson:"tokenSymbol"`
	TokenDecimals   int    `json:"tokenDecimals" bson:"tokenDecimals"`
	TokenName       string `json:"tokenName" bson:"tokenName"`
	TransferType    string `json:"transferType" bson:"transferType"`
	TokenStandard   string `json:"tokenStandard,omitempty" bson:"tokenStandard,omitempty"`
	TokenID         string `json:"tokenID,omitempty" bson:"tokenID,omitempty"`
}

// NFTBalance is one row of GET /address/:addr/nfts: the holder's
// claim on a specific (collection, tokenID) pair, joined with the
// collection-level contractCode metadata AND the per-tokenID
// tokenMetadata document (Phase 3b) so the wallet can render a name
// + thumbnail + attributes in a single response.
//
// The collection-level fields are renamed to make the projection
// unambiguous (collectionName vs the per-token name from
// tokenMetadata).
type NFTBalance struct {
	ContractAddress  string           `json:"contractAddress" bson:"contractAddress"`
	HolderAddress    string           `json:"holderAddress" bson:"holderAddress"`
	TokenID          string           `json:"tokenID" bson:"tokenID"`
	TokenStandard    string           `json:"tokenStandard" bson:"tokenStandard"`
	Balance          string           `json:"balance" bson:"balance"`
	BlockNumber      string           `json:"blockNumber,omitempty" bson:"blockNumber,omitempty"`
	UpdatedAt        string           `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
	CollectionName   string           `json:"collectionName,omitempty" bson:"collectionName,omitempty"`
	CollectionSymbol string           `json:"collectionSymbol,omitempty" bson:"collectionSymbol,omitempty"`
	Name             string           `json:"name,omitempty" bson:"name,omitempty"`
	Description      string           `json:"description,omitempty" bson:"description,omitempty"`
	Image            string           `json:"image,omitempty" bson:"image,omitempty"`
	ExternalURL      string           `json:"externalURL,omitempty" bson:"externalURL,omitempty"`
	Attributes       []TokenAttribute `json:"attributes,omitempty" bson:"attributes,omitempty"`
}

// NFTBalancesResponse is the wrapper for GET /address/:addr/nfts. Mirrors
// the TokenBalancesResponse shape so wallet clients can share their
// list-payload handling.
type NFTBalancesResponse struct {
	Address string       `json:"address"`
	NFTs    []NFTBalance `json:"nfts"`
	Count   int          `json:"count"`
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
	GenesisContract bool   `json:"genesisContract,omitempty"`
}
