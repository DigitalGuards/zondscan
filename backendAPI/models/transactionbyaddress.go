package models

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionByAddress struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	InOut     int                `bson:"inOut" json:"InOut"`
	TxType    string             `bson:"txType" json:"TxType"`
	Address   string             `json:"Address" bson:"Address"`
	From      string             `bson:"from" json:"From"`
	To        string             `bson:"to" json:"To"`
	TxHash    string             `bson:"txHash" json:"TxHash"`
	TimeStamp string             `bson:"timeStamp" json:"TimeStamp"`
	Amount    float64            `bson:"amount" json:"-"`
	PaidFees  float64            `bson:"paidFees" json:"-"`
	// AmountWei / PaidFeesWei carry the exact value as a base-10 wei integer
	// string (e.g. "3300000000000000000"). The syncer writes these so the API
	// can render QRL without ever routing the value through a float64, which
	// can't represent 3.3 and used to leak as "3.299999999999999822". Older
	// documents predate these fields; MarshalJSON falls back to the float
	// columns above when they are empty.
	AmountWei   string `bson:"amountWei" json:"-"`
	PaidFeesWei string `bson:"paidFeesWei" json:"-"`
	BlockNumber string `bson:"blockNumber" json:"BlockNumber"`
}

// weiPerQuanta is 10^18: one QRL expressed in wei. Kept as a big.Int so the
// wei -> QRL conversion stays in exact integer/rational arithmetic and never
// touches a float64. 10^18 fits in an int64 (max ~9.22e18), so a literal is
// cheaper and clearer than computing it with Exp at init.
var weiPerQuanta = big.NewInt(1_000_000_000_000_000_000)

// quantaFromWei converts a raw wei amount, given as a base-10 integer string,
// into an exact QRL decimal string with 18 fractional digits. Because the
// math is done with big.Int/big.Rat, 3300000000000000000 wei renders as
// "3.300000000000000000" — never "3.2999999999999998xx" — and arbitrarily
// large values keep every digit. Returns ("", false) when s is not a valid
// integer (including the empty string) so callers can fall back.
func quantaFromWei(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	wei, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return "", false
	}
	// SetFrac models wei/10^18 as an exact rational; FloatString(18) renders
	// it at exactly 18 dp with no rounding (10^18 has 18 digits).
	return new(big.Rat).SetFrac(wei, weiPerQuanta).FloatString(18), true
}

// formatFloat is the legacy fallback for documents written before the exact
// wei fields existed. The stored float64 already lost precision at sync time,
// but FormatFloat with prec -1 emits the shortest decimal that round-trips to
// it ("3.3", not "3.299999999999999822"), and re-rendering that through
// big.Rat yields the same fixed-18-dp shape as the exact path. Best-effort:
// spot-on for typical amounts, still float-bounded for very large legacy
// values, which only an upstream re-sync (populating amountWei) can fully fix.
func formatFloat(f float64) string {
	shortest := strconv.FormatFloat(f, 'f', -1, 64)
	if r, ok := new(big.Rat).SetString(shortest); ok {
		return r.FloatString(18)
	}
	return shortest
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

// MarshalJSON implements custom JSON marshaling. Amount and PaidFees are
// emitted as exact QRL decimal strings, preferring the raw-wei integer fields
// and falling back to the legacy float columns for documents that predate
// them.
func (t TransactionByAddress) MarshalJSON() ([]byte, error) {
	type Alias TransactionByAddress

	amount, ok := quantaFromWei(t.AmountWei)
	if !ok {
		amount = formatFloat(t.Amount)
	}
	paidFees, ok := quantaFromWei(t.PaidFeesWei)
	if !ok {
		paidFees = formatFloat(t.PaidFees)
	}

	return json.Marshal(struct {
		Alias
		Amount      string `json:"Amount"`
		PaidFees    string `json:"PaidFees"`
		BlockNumber string `json:"BlockNumber"`
	}{
		Alias:       Alias(t),
		Amount:      amount,
		PaidFees:    paidFees,
		BlockNumber: formatBlockNumber(t.BlockNumber),
	})
}
