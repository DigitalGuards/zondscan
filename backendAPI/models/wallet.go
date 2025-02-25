package models

type WalletCount struct {
	ID    string `bson:"_id"`
	Count int64  `bson:"count"`
}
