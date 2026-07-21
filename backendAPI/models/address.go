package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Address struct {
	ObjectId primitive.ObjectID `bson:"_id"`
	ID       string             `json:"id"` // Changed from []byte to string
	Balance  float64            `json:"balance"`
	Nonce    uint64             `json:"nonce"`
}

// RichlistEntry is one row of the /richlist response. Deliberately separate
// from Address: that struct serves /address/aggregate (and leaks the Mongo
// ObjectId), while the richlist table needs the enrichment columns below and
// must not expose internal ids.
type RichlistEntry struct {
	ID            string  `json:"id" bson:"id"`
	Balance       float64 `json:"balance" bson:"balance"`
	IsContract    bool    `json:"isContract" bson:"isContract"`
	FirstSeen     int64   `json:"firstSeen" bson:"firstSeen"`
	SupplyPercent float64 `json:"supplyPercent" bson:"supplyPercent"`
}
