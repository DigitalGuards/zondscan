package models

import "QRLtoMongoDB-PoS/bitfield"

type Address struct {
	ID       string       `json:"id"`
	Bitfield bitfield.Big `json:"bitfield"`
}
