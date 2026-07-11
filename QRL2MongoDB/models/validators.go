package models

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

// Beacon chain API validator models
type BeaconValidatorResponse struct {
	ValidatorList []BeaconValidator `json:"validatorList"`
	NextPageToken string            `json:"nextPageToken"`
	TotalSize     int               `json:"totalSize"`
}

type BeaconValidator struct {
	Index     string           `json:"index"`
	Validator ValidatorDetails `json:"validator"`
}

type ValidatorDetails struct {
	PublicKey                  string `json:"publicKey"`
	WithdrawalCredentials      string `json:"withdrawalCredentials"`
	EffectiveBalance           string `json:"effectiveBalance"`
	Slashed                    bool   `json:"slashed"`
	ActivationEligibilityEpoch string `json:"activationEligibilityEpoch"`
	ActivationEpoch            string `json:"activationEpoch"`
	ExitEpoch                  string `json:"exitEpoch"`
	WithdrawableEpoch          string `json:"withdrawableEpoch"`
}

// Helper methods for base64 to hex conversion
func Base64ToHex(b64 string) string {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(data)
}

// EpochInfo represents the current epoch state from beacon chain
type EpochInfo struct {
	ID             string `bson:"_id" json:"_id"`                       // Always "current"
	HeadEpoch      string `bson:"headEpoch" json:"headEpoch"`           // Current head epoch
	HeadSlot       string `bson:"headSlot" json:"headSlot"`             // Current head slot
	FinalizedEpoch string `bson:"finalizedEpoch" json:"finalizedEpoch"` // Last finalized epoch
	JustifiedEpoch string `bson:"justifiedEpoch" json:"justifiedEpoch"` // Last justified epoch
	FinalizedSlot  string `bson:"finalizedSlot" json:"finalizedSlot"`   // Last finalized slot
	JustifiedSlot  string `bson:"justifiedSlot" json:"justifiedSlot"`   // Last justified slot
	GenesisTime    string `bson:"genesisTime" json:"genesisTime"`       // Genesis timestamp
	UpdatedAt      int64  `bson:"updatedAt" json:"updatedAt"`           // Last update timestamp
}

// BeaconChainHeadResponse represents the response from beacon chain head endpoint
type BeaconChainHeadResponse struct {
	HeadSlot                   string `json:"headSlot"`
	HeadEpoch                  string `json:"headEpoch"`
	HeadBlockRoot              string `json:"headBlockRoot"`
	FinalizedSlot              string `json:"finalizedSlot"`
	FinalizedEpoch             string `json:"finalizedEpoch"`
	FinalizedBlockRoot         string `json:"finalizedBlockRoot"`
	JustifiedSlot              string `json:"justifiedSlot"`
	JustifiedEpoch             string `json:"justifiedEpoch"`
	JustifiedBlockRoot         string `json:"justifiedBlockRoot"`
	PreviousJustifiedSlot      string `json:"previousJustifiedSlot"`
	PreviousJustifiedEpoch     string `json:"previousJustifiedEpoch"`
	PreviousJustifiedBlockRoot string `json:"previousJustifiedBlockRoot"`
	OptimisticStatus           bool   `json:"optimisticStatus"`
}

// ValidatorHistoryRecord represents historical validator statistics per epoch
type ValidatorHistoryRecord struct {
	ID              string `bson:"_id,omitempty" json:"_id,omitempty"`
	Epoch           string `bson:"epoch" json:"epoch"`
	Timestamp       int64  `bson:"timestamp" json:"timestamp"`
	ValidatorsCount int    `bson:"validatorsCount" json:"validatorsCount"`
	ActiveCount     int    `bson:"activeCount" json:"activeCount"`
	PendingCount    int    `bson:"pendingCount" json:"pendingCount"`
	ExitedCount     int    `bson:"exitedCount" json:"exitedCount"`
	SlashedCount    int    `bson:"slashedCount" json:"slashedCount"`
	TotalStaked     string `bson:"totalStaked" json:"totalStaked"` // Sum of effective balances
}

// ValidatorDocument is the per-validator MongoDB document.
// Each validator is stored as its own document with _id == validator index.
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

// GetValidatorStatus computes the validator status based on current epoch
func GetValidatorStatus(activationEpoch, exitEpoch string, slashed bool, currentEpoch int64) string {
	activation, _ := strconv.ParseInt(activationEpoch, 10, 64)
	exit, _ := strconv.ParseInt(exitEpoch, 10, 64)

	if slashed {
		return "slashed"
	}
	if activation > currentEpoch {
		return "pending"
	}
	if exit <= currentEpoch {
		return "exited"
	}
	return "active"
}
