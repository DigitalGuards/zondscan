package models

import (
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionByAddress struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	InOut       int               `bson:"inOut" json:"InOut"`
	TxType      string            `bson:"txType" json:"TxType"`
	Address     string            `json:"Address" bson:"Address"`
	From        string            `bson:"from" json:"From"`
	To          string            `bson:"to" json:"To"`
	TxHash      string            `bson:"txHash" json:"TxHash"`
	TimeStamp   string            `bson:"timeStamp" json:"TimeStamp"`
	Amount      float64           `bson:"amount" json:"-"`
	PaidFees    float64           `bson:"paidFees" json:"-"`
	BlockNumber uint64            `json:"BlockNumber"`
}

func formatFloat(f float64) string {
	// Use %.18f to show all 18 decimal places for wei
	return fmt.Sprintf("%.18f", f)
}

// MarshalJSON implements custom JSON marshaling
func (t TransactionByAddress) MarshalJSON() ([]byte, error) {
	type Alias TransactionByAddress
	return json.Marshal(struct {
		Alias
		Amount   string `json:"Amount"`
		PaidFees string `json:"PaidFees"`
	}{
		Alias:    Alias(t),
		Amount:   formatFloat(t.Amount),
		PaidFees: formatFloat(t.PaidFees),
	})
}
