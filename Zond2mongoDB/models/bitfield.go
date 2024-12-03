package models

import "Zond2mongoDB/bitfield"

type Address struct {
	ID       string       `json:"id"`
	Bitfield bitfield.Big `json:"bitfield"`
}
