package models

import "time"

// ContractExplainChallenge is a short-lived, single-use authorization record
// for creator-triggered AI explanation regeneration. The exact signed message
// is persisted so every backend instance verifies the same bytes.
type ContractExplainChallenge struct {
	ID              string    `bson:"_id"`
	Version         string    `bson:"version"`
	Action          string    `bson:"action"`
	Origin          string    `bson:"origin"`
	ChainID         string    `bson:"chainId"`
	Method          string    `bson:"method"`
	Route           string    `bson:"route"`
	ContractAddress string    `bson:"contractAddress"`
	SignerAddress   string    `bson:"signerAddress"`
	Message         []byte    `bson:"message"`
	IssuedAt        time.Time `bson:"issuedAt"`
	ExpiresAt       time.Time `bson:"expiresAt"`
}
