package models

import "errors"

// Structural evidence failures resolve the comparison with a missing marker.
var ErrStructuralSlashingEvidence = errors.New("structural slashing evidence error")

type ValidatorSetChange struct {
	ID             string `bson:"_id" json:"id"`
	Epoch          int64  `bson:"epoch" json:"epoch"`
	Kind           string `bson:"kind" json:"kind"`
	ValidatorIndex string `bson:"validatorIndex" json:"validatorIndex"`
	Pubkey         string `bson:"pubkey" json:"pubkey"`
}

type SlashingEvent struct {
	ID               string   `bson:"_id" json:"id"`
	Slot             int64    `bson:"slot" json:"slot"`
	Epoch            int64    `bson:"epoch" json:"epoch"`
	Type             string   `bson:"type" json:"type"`
	ValidatorIndices []string `bson:"validatorIndices" json:"validatorIndices"`
	Whistleblower    string   `bson:"whistleblower" json:"whistleblower"`
	ConflictingRoots []string `bson:"conflictingRoots" json:"conflictingRoots"`
}

// Only slashed records are needed to compare adjacent first-slot states.
type SlashedValidator struct {
	Index           string `bson:"index"`
	Pubkey          string `bson:"pubkey"`
	ActivationEpoch uint64 `bson:"activationEpoch"`
}

type SlashingSnapshot struct {
	Epoch           int64              `bson:"epoch"`
	Validators      []SlashedValidator `bson:"validators"`
	Resolved        bool               `bson:"resolved"`
	EvidenceMissing bool               `bson:"evidenceMissing,omitempty"`
}
