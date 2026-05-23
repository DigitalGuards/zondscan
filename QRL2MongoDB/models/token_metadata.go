package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// TokenMetadata stores off-chain per-tokenID NFT metadata for an
// (ERC-721 / ERC-1155) collection. One row per (contractAddress, tokenID).
//
// Phase 3b writes one stub row on every NFT transfer via $setOnInsert so
// the metadata fetcher service has a work queue to chew through; the
// fetcher then resolves the per-token URI (tokenURI(id) / uri(id)) via
// the configured IPFS gateway, parses the JSON, and writes back into
// the same row.
//
// All resolved fields use omitempty so stubs stay clean until populated
// and the fetcher's "preserve last-good" semantics work cleanly when a
// later refresh fails.
type TokenMetadata struct {
	ID              primitive.ObjectID `bson:"_id"`
	ContractAddress string             `bson:"contractAddress" json:"contractAddress"`
	TokenID         string             `bson:"tokenID" json:"tokenID"`
	TokenStandard   string             `bson:"tokenStandard,omitempty" json:"tokenStandard,omitempty"`
	URI             string             `bson:"uri,omitempty" json:"uri,omitempty"`

	Name        string           `bson:"name,omitempty" json:"name,omitempty"`
	Description string           `bson:"description,omitempty" json:"description,omitempty"`
	Image       string           `bson:"image,omitempty" json:"image,omitempty"`
	ExternalURL string           `bson:"externalURL,omitempty" json:"externalURL,omitempty"`
	Attributes  []TokenAttribute `bson:"attributes,omitempty" json:"attributes,omitempty"`

	FetchedAt  string `bson:"fetchedAt,omitempty" json:"fetchedAt,omitempty"`
	FetchError string `bson:"fetchError,omitempty" json:"fetchError,omitempty"`
	UpdatedAt  string `bson:"updatedAt" json:"updatedAt"`
}

// TokenAttribute is one OpenSea-style trait. The spec is loose about the
// value's type (often string, sometimes number, sometimes object), so the
// parser stringifies any non-object value at fetch time for storage
// uniformity; consumers can re-parse as needed.
type TokenAttribute struct {
	TraitType   string `bson:"trait_type" json:"trait_type"`
	Value       string `bson:"value" json:"value"`
	DisplayType string `bson:"display_type,omitempty" json:"display_type,omitempty"`
}
