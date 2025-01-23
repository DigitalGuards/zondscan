package models

import "Zond2mongoDB/bitfield"

type BitfieldAddress struct {
	ID       string       `json:"id"`
	Bitfield bitfield.Big `json:"bitfield"`
}
