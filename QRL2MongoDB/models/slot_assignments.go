package models

const (
	SlotsPerEpoch          = int64(8)
	SecondsPerSlot         = int64(60)
	SlotAssignmentCursorID = "slot_assignments"
)

// SlotAssignment records the consensus duty even when the slot has no block.
// Slot zero has no proposer duty and carries empty validator fields.
type SlotAssignment struct {
	Epoch          int64  `bson:"epoch"`
	Slot           int64  `bson:"slot"`
	ValidatorIndex string `bson:"validatorIndex"`
	Pubkey         string `bson:"pubkey"`
}

// SlotAssignmentCursor advances only after all assignments have been upserted.
type SlotAssignmentCursor struct {
	NextEpoch           int64  `bson:"nextEpoch"`
	GenesisTime         int64  `bson:"genesisTime"`
	FirstAvailableEpoch *int64 `bson:"firstAvailableEpoch,omitempty"`
	UnavailableEpochs   int64  `bson:"unavailableEpochs"`
}
