package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionByAddress struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	InOut       int                `bson:"inOut" json:"InOut"`
	TxType      string             `bson:"txType" json:"TxType"`
	Address     string             `json:"Address" bson:"Address"`
	From        string             `bson:"from" json:"From"`
	To          string             `bson:"to" json:"To"`
	TxHash      string             `bson:"txHash" json:"TxHash"`
	TimeStamp   string             `bson:"timeStamp" json:"TimeStamp"`
	Amount      float64            `bson:"amount" json:"-"`
	PaidFees    float64            `bson:"paidFees" json:"-"`
	BlockNumber string             `bson:"blockNumber" json:"BlockNumber"`
}

func formatFloat(f float64) string {
	// Use %.18f to show all 18 decimal places for wei
	return fmt.Sprintf("%.18f", f)
}

func formatBlockNumber(blockNum string) string {
	if blockNum == "" {
		return ""
	}
	if strings.HasPrefix(blockNum, "0x") {
		num, err := strconv.ParseUint(blockNum[2:], 16, 64)
		if err != nil {
			return blockNum
		}
		return strconv.FormatUint(num, 10)
	}
	return blockNum
}

// MarshalJSON implements custom JSON marshaling
func (t TransactionByAddress) MarshalJSON() ([]byte, error) {
	type Alias TransactionByAddress
	return json.Marshal(struct {
		Alias
		Amount      string `json:"Amount"`
		PaidFees    string `json:"PaidFees"`
		BlockNumber string `json:"BlockNumber"`
	}{
		Alias:       Alias(t),
		Amount:      formatFloat(t.Amount),
		PaidFees:    formatFloat(t.PaidFees),
		BlockNumber: formatBlockNumber(t.BlockNumber),
	})
}
