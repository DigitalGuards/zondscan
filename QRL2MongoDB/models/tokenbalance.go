package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TokenBalance represents a holder's balance for a specific token.
//
// Phase 2 extends the original (contractAddress, holderAddress) shape with
// TokenID + TokenStandard so the same collection can store ERC-721
// single-owner rows and ERC-1155 per-id quantities alongside the existing
// ERC-20 rows. TokenID is the decimal-string form of the on-chain uint256.
//
// For ERC-20 rows, TokenID and TokenStandard are emitted as `omitempty`,
// legacy rows (pre-migration) lack the field entirely, which Mongo treats as
// null and which still upholds the new (contract, holder, tokenID) unique by
// extension of the prior (contract, holder) unique.
type TokenBalance struct {
	ID              primitive.ObjectID `bson:"_id"`
	ContractAddress string             `bson:"contractAddress" json:"contractAddress"`
	HolderAddress   string             `bson:"holderAddress" json:"holderAddress"`
	Balance         string             `bson:"balance" json:"balance"`         // decimal string for NFT counts, hex/decimal for ERC-20
	BlockNumber     string             `bson:"blockNumber" json:"blockNumber"` // hex string
	UpdatedAt       string             `bson:"updatedAt" json:"updatedAt"`
	TokenID         string             `bson:"tokenID,omitempty" json:"tokenID,omitempty"`             // decimal uint256, empty for ERC-20
	TokenStandard   string             `bson:"tokenStandard,omitempty" json:"tokenStandard,omitempty"` // ERC-20 | ERC-721 | ERC-1155
}
