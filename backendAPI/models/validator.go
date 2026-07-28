package models

// ValidatorDocument is the per-validator MongoDB document written by the syncer.
// _id is the validator index (decimal string).
type ValidatorDocument struct {
	ID                         string `bson:"_id" json:"_id"`
	PublicKeyHex               string `bson:"publicKeyHex" json:"publicKeyHex"`
	WithdrawalCredentialsHex   string `bson:"withdrawalCredentialsHex" json:"withdrawalCredentialsHex"`
	EffectiveBalance           string `bson:"effectiveBalance" json:"effectiveBalance"`
	Slashed                    bool   `bson:"slashed" json:"slashed"`
	ActivationEligibilityEpoch string `bson:"activationEligibilityEpoch" json:"activationEligibilityEpoch"`
	ActivationEpoch            string `bson:"activationEpoch" json:"activationEpoch"`
	ExitEpoch                  string `bson:"exitEpoch" json:"exitEpoch"`
	WithdrawableEpoch          string `bson:"withdrawableEpoch" json:"withdrawableEpoch"`
	SlotNumber                 string `bson:"slotNumber" json:"slotNumber"`
	IsLeader                   bool   `bson:"isLeader" json:"isLeader"`
	Epoch                      string `bson:"epoch" json:"epoch"`
	UpdatedAt                  string `bson:"updatedAt" json:"updatedAt"`
}

// ValidatorResponse represents the API response format
type ValidatorResponse struct {
	Validators     []Validator `json:"validators"`
	ValidatorCount int         `json:"validatorCount"`
	Epoch          string      `json:"epoch"`
	TotalStaked    string      `json:"totalStaked"`
}

// Validator represents a single validator in the API response
type Validator struct {
	Index                    string `json:"index"`                    // Validator index
	Address                  string `json:"address"`                  // Public key in hex format
	WithdrawalCredentialsHex string `json:"withdrawalCredentialsHex"` // Withdrawal credentials in hex format
	Status                   string `json:"status"`                   // active, pending, exited, slashed
	Age                      int64  `json:"age"`                      // Age in epochs
	StakedAmount             string `json:"stakedAmount"`             // Amount staked
	IsActive                 bool   `json:"isActive"`                 // Whether the validator is currently active
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

// EpochDetailBlock represents a single slot within an epoch detail view
type EpochDetailBlock struct {
	Slot         int64  `json:"slot"`
	Status       string `json:"status"` // proposed or missed
	Timestamp    string `json:"timestamp,omitempty"`
	Proposer     string `json:"proposer,omitempty"`
	Transactions int    `json:"transactions"`
	GasUsed      string `json:"gasUsed,omitempty"`
	BlockHash    string `json:"blockHash,omitempty"`
}

// EpochDetailResponse represents the full epoch detail API response
type EpochDetailResponse struct {
	Epoch           string             `json:"epoch"`
	Timestamp       int64              `json:"timestamp"`
	Status          string             `json:"status"`
	ValidatorsCount int                `json:"validatorsCount"`
	ActiveCount     int                `json:"activeCount"`
	PendingCount    int                `json:"pendingCount"`
	ExitedCount     int                `json:"exitedCount"`
	SlashedCount    int                `json:"slashedCount"`
	TotalStaked     string             `json:"totalStaked"`
	StartSlot       int64              `json:"startSlot"`
	EndSlot         int64              `json:"endSlot"`
	Blocks          []EpochDetailBlock `json:"blocks"`
	FinalizedEpoch  string             `json:"finalizedEpoch"`
	JustifiedEpoch  string             `json:"justifiedEpoch"`
	HeadEpoch       string             `json:"headEpoch"`
	ProposedCount   int                `json:"proposedCount"`
	MissedCount     int                `json:"missedCount"`
}

// EpochListItem represents a single epoch in the paginated listing
type EpochListItem struct {
	Epoch           string `json:"epoch"`
	Timestamp       int64  `json:"timestamp"`
	Status          string `json:"status"` // finalized, justified, pending
	ValidatorsCount int    `json:"validatorsCount"`
	ActiveCount     int    `json:"activeCount"`
	TotalStaked     string `json:"totalStaked"`
}

// EpochsResponse represents the paginated epochs API response
type EpochsResponse struct {
	Epochs         []EpochListItem `json:"epochs"`
	Total          int64           `json:"total"`
	FinalizedEpoch string          `json:"finalizedEpoch"`
	JustifiedEpoch string          `json:"justifiedEpoch"`
}
