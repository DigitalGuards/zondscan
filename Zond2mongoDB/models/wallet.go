package models

import (
	"Zond2mongoDB/bitfield"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Wallet struct {
	_id    primitive.ObjectID `json:"_id,omitempty"`
	Id     string             `json:"id,omitempty"`
	Amount int                `json:"amount,omitempty"`
	Paged  bitfield.Big       `json:"bitfield"`
}
