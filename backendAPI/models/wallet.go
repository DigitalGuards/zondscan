package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type WalletCount struct {
	ID    primitive.ObjectID `bson:"_id,omitempty"`
	Count int64              `bson:"count"`
}
