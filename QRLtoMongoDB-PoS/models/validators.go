package models

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
