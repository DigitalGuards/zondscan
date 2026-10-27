package models

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

type ValidatorActivityQuery struct {
	Page, Limit          int
	FromEpoch, ToEpoch   *int64
	Kind, ValidatorIndex string
}

type SlashingsResponse struct {
	Events                []SlashingEvent `json:"events"`
	Total                 int64           `json:"total"`
	Page                  int             `json:"page"`
	Limit                 int             `json:"limit"`
	SlashedValidatorCount int64           `json:"slashedValidatorCount"`
	BackfillComplete      bool            `json:"backfillComplete"`
}

type ValidatorSetChangesResponse struct {
	Changes          []ValidatorSetChange `json:"changes"`
	Total            int64                `json:"total"`
	Page             int                  `json:"page"`
	Limit            int                  `json:"limit"`
	BackfillComplete bool                 `json:"backfillComplete"`
}
