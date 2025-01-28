package models

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

// Legacy validator models
type Validators struct {
	Jsonrpc         string          `json:"jsonrpc"`
	ID              int             `json:"id"`
	ResultValidator ResultValidator `json:"result"`
}

type ValidatorsBySlotNumber struct {
	SlotNumber int      `json:"slotNumber"`
	Leader     string   `json:"leader"`
	Attestors  []string `json:"attestors"`
}

type ResultValidator struct {
	Epoch                  int                      `json:"epoch"`
	ValidatorsBySlotNumber []ValidatorsBySlotNumber `json:"validatorsBySlotNumber"`
}

// New beacon chain API validator models
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

// MongoDB storage model
type ValidatorStorage struct {
	ID         string            `bson:"_id" json:"_id"`
	Epoch      string            `bson:"epoch" json:"epoch"`           // Stored as decimal
	Validators []ValidatorRecord `bson:"validators" json:"validators"` // All validator details
	UpdatedAt  string            `bson:"updatedAt" json:"updatedAt"`   // Unix timestamp
}

type ValidatorRecord struct {
	Index                      string `bson:"index" json:"index"`                                       // Decimal string
	PublicKeyHex               string `bson:"publicKeyHex" json:"publicKeyHex"`                         // Converted from base64 to hex
	WithdrawalCredentialsHex   string `bson:"withdrawalCredentialsHex" json:"withdrawalCredentialsHex"` // Converted from base64 to hex
	EffectiveBalance           string `bson:"effectiveBalance" json:"effectiveBalance"`                 // Decimal string
	Slashed                    bool   `bson:"slashed" json:"slashed"`
	ActivationEligibilityEpoch string `bson:"activationEligibilityEpoch" json:"activationEligibilityEpoch"` // Decimal string
	ActivationEpoch            string `bson:"activationEpoch" json:"activationEpoch"`                       // Decimal string
	ExitEpoch                  string `bson:"exitEpoch" json:"exitEpoch"`                                   // Decimal string
	WithdrawableEpoch          string `bson:"withdrawableEpoch" json:"withdrawableEpoch"`                   // Decimal string
	SlotNumber                 string `bson:"slotNumber" json:"slotNumber"`                                 // Decimal string
	IsLeader                   bool   `bson:"isLeader" json:"isLeader"`
}

// Helper methods for base64 to hex conversion
func Base64ToHex(b64 string) string {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(data)
}

// Frontend validator models
type ValidatorResponse struct {
	Validators  []ValidatorRecord `json:"validators"`
	TotalStaked string            `json:"totalStaked"` // Decimal string
	Epoch       string            `json:"epoch"`       // Decimal string
}

type Validator struct {
	Address      string  `json:"address"`
	Uptime       float64 `json:"uptime"`
	Age          int     `json:"age"`
	StakedAmount string  `json:"stakedAmount"`
	IsActive     bool    `json:"isActive"`
}

// Helper methods for ValidatorDetails
func (v *ValidatorDetails) GetEffectiveBalanceGwei() string {
	return v.EffectiveBalance
}

func (v *ValidatorDetails) IsActive(currentEpoch int64) bool {
	activationEpoch, _ := strconv.ParseInt(v.ActivationEpoch, 10, 64)
	exitEpoch, _ := strconv.ParseInt(v.ExitEpoch, 10, 64)
	return activationEpoch <= currentEpoch && currentEpoch < exitEpoch
}

func (v *ValidatorDetails) GetAge(currentEpoch int64) int64 {
	activationEpoch, _ := strconv.ParseInt(v.ActivationEpoch, 10, 64)
	if activationEpoch > currentEpoch {
		return 0
	}
	return currentEpoch - activationEpoch
}

// Helper method to convert public key to address format
func (v *ValidatorDetails) ToAddress() string {
	return v.PublicKey
}
