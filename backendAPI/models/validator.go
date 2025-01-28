package models

// ValidatorStorage represents the validator document in MongoDB
type ValidatorStorage struct {
	ID         string            `bson:"_id" json:"_id"`
	Epoch      string            `bson:"epoch" json:"epoch"`           // Stored as hex
	Validators []ValidatorRecord `bson:"validators" json:"validators"` // All validator details
	UpdatedAt  string            `bson:"updatedAt" json:"updatedAt"`   // Timestamp in hex
}

// ValidatorRecord represents a single validator's data
type ValidatorRecord struct {
	Index                      string `bson:"index" json:"index"`                                       // Stored as hex
	PublicKeyHex               string `bson:"publicKeyHex" json:"publicKeyHex"`                         // Converted from base64 to hex
	WithdrawalCredentialsHex   string `bson:"withdrawalCredentialsHex" json:"withdrawalCredentialsHex"` // Converted from base64 to hex
	EffectiveBalance           string `bson:"effectiveBalance" json:"effectiveBalance"`                 // Stored as hex
	Slashed                    bool   `bson:"slashed" json:"slashed"`
	ActivationEligibilityEpoch string `bson:"activationEligibilityEpoch" json:"activationEligibilityEpoch"` // Stored as hex
	ActivationEpoch            string `bson:"activationEpoch" json:"activationEpoch"`                       // Stored as hex
	ExitEpoch                  string `bson:"exitEpoch" json:"exitEpoch"`                                   // Stored as hex
	WithdrawableEpoch          string `bson:"withdrawableEpoch" json:"withdrawableEpoch"`                   // Stored as hex
	SlotNumber                 string `bson:"slotNumber" json:"slotNumber"`                                 // Stored as hex
	IsLeader                   bool   `bson:"isLeader" json:"isLeader"`
}

// ValidatorResponse represents the API response format
type ValidatorResponse struct {
	Validators  []Validator `json:"validators"`
	TotalStaked string      `json:"totalStaked"` // Total amount staked in hex
}

// Validator represents a single validator in the API response
type Validator struct {
	Address      string  `json:"address"`      // Public key in hex format
	Uptime       float64 `json:"uptime"`       // Validator uptime percentage
	Age          int64   `json:"age"`          // Age in epochs
	StakedAmount string  `json:"stakedAmount"` // Amount staked in hex
	IsActive     bool    `json:"isActive"`     // Whether the validator is currently active
}
