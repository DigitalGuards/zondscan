package models

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
	Epoch         string           `json:"epoch"`
	ValidatorList []BeaconValidator `json:"validatorList"`
}

type BeaconValidator struct {
	Index     string           `json:"index"`
	Validator ValidatorDetails `json:"validator"`
}

type ValidatorDetails struct {
	PublicKey                  string `json:"publicKey"`
	WithdrawalCredentials      string `json:"withdrawalCredentials"`
	EffectiveBalance          string `json:"effectiveBalance"`
	Slashed                   bool   `json:"slashed"`
	ActivationEligibilityEpoch string `json:"activationEligibilityEpoch"`
	ActivationEpoch           string `json:"activationEpoch"`
	ExitEpoch                 string `json:"exitEpoch"`
	WithdrawableEpoch         string `json:"withdrawableEpoch"`
}
