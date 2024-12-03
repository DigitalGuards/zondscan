package models

import (
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
	Epoch         string            `json:"epoch"`
	ValidatorList []BeaconValidator `json:"validatorList"`
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

// Frontend validator models
type ValidatorResponse struct {
	Validators  []Validator `json:"validators"`
	TotalStaked string      `json:"totalStaked"`
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
