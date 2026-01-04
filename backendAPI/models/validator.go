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
	Index        string `json:"index"`        // Validator index
	Address      string `json:"address"`      // Public key in hex format
	Status       string `json:"status"`       // active, pending, exited, slashed
	Age          int64  `json:"age"`          // Age in epochs
	StakedAmount string `json:"stakedAmount"` // Amount staked
	IsActive     bool   `json:"isActive"`     // Whether the validator is currently active
}

// EpochInfo represents the current epoch state
type EpochInfo struct {
	ID             string `bson:"_id" json:"_id"`
	HeadEpoch      string `bson:"headEpoch" json:"headEpoch"`
	HeadSlot       string `bson:"headSlot" json:"headSlot"`
	FinalizedEpoch string `bson:"finalizedEpoch" json:"finalizedEpoch"`
	JustifiedEpoch string `bson:"justifiedEpoch" json:"justifiedEpoch"`
	FinalizedSlot  string `bson:"finalizedSlot" json:"finalizedSlot"`
	JustifiedSlot  string `bson:"justifiedSlot" json:"justifiedSlot"`
	UpdatedAt      int64  `bson:"updatedAt" json:"updatedAt"`
}

// EpochInfoResponse represents the API response for epoch information
type EpochInfoResponse struct {
	HeadEpoch       string `json:"headEpoch"`
	HeadSlot        string `json:"headSlot"`
	FinalizedEpoch  string `json:"finalizedEpoch"`
	JustifiedEpoch  string `json:"justifiedEpoch"`
	SlotsPerEpoch   int    `json:"slotsPerEpoch"`
	SecondsPerSlot  int    `json:"secondsPerSlot"`
	SlotInEpoch     int64  `json:"slotInEpoch"`
	TimeToNextEpoch int64  `json:"timeToNextEpoch"` // Seconds until next epoch
	UpdatedAt       int64  `json:"updatedAt"`
}

// ValidatorHistoryRecord represents historical validator statistics
type ValidatorHistoryRecord struct {
	ID              string `bson:"_id,omitempty" json:"_id,omitempty"`
	Epoch           string `bson:"epoch" json:"epoch"`
	Timestamp       int64  `bson:"timestamp" json:"timestamp"`
	ValidatorsCount int    `bson:"validatorsCount" json:"validatorsCount"`
	ActiveCount     int    `bson:"activeCount" json:"activeCount"`
	PendingCount    int    `bson:"pendingCount" json:"pendingCount"`
	ExitedCount     int    `bson:"exitedCount" json:"exitedCount"`
	SlashedCount    int    `bson:"slashedCount" json:"slashedCount"`
	TotalStaked     string `bson:"totalStaked" json:"totalStaked"`
}

// ValidatorHistoryResponse represents the API response for validator history
type ValidatorHistoryResponse struct {
	History []ValidatorHistoryRecord `json:"history"`
}

// ValidatorDetailResponse represents detailed information about a single validator
type ValidatorDetailResponse struct {
	Index                      string `json:"index"`
	PublicKeyHex               string `json:"publicKeyHex"`
	WithdrawalCredentialsHex   string `json:"withdrawalCredentialsHex"`
	EffectiveBalance           string `json:"effectiveBalance"`
	Slashed                    bool   `json:"slashed"`
	ActivationEligibilityEpoch string `json:"activationEligibilityEpoch"`
	ActivationEpoch            string `json:"activationEpoch"`
	ExitEpoch                  string `json:"exitEpoch"`
	WithdrawableEpoch          string `json:"withdrawableEpoch"`
	Status                     string `json:"status"`
	Age                        int64  `json:"age"`
	CurrentEpoch               string `json:"currentEpoch"`
}

// ValidatorStatsResponse represents aggregated validator statistics
type ValidatorStatsResponse struct {
	TotalValidators int    `json:"totalValidators"`
	ActiveCount     int    `json:"activeCount"`
	PendingCount    int    `json:"pendingCount"`
	ExitedCount     int    `json:"exitedCount"`
	SlashedCount    int    `json:"slashedCount"`
	TotalStaked     string `json:"totalStaked"`
	CurrentEpoch    string `json:"currentEpoch"`
}
