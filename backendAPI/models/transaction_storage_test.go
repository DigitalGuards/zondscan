package models

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestIndexedTransactionReceiptAndLegacyCalldataDecode(t *testing.T) {
	doc, err := bson.Marshal(bson.M{"data": "0x12345678", "status": "0x0", "gas": "0x186a0", "gasUsed": "0x5208", "effectiveGasPrice": "0x7"})
	if err != nil {
		t.Fatal(err)
	}
	var tx Transaction
	if err := bson.Unmarshal(doc, &tx); err != nil {
		t.Fatal(err)
	}
	if tx.Data != "0x12345678" || tx.Status != "0x0" || tx.GasUsed != "0x5208" || tx.EffectiveGasPrice != "0x7" {
		t.Fatalf("indexed receipt lost: %+v", tx)
	}
	encoded, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["input"] != "0x12345678" || wire["data"] != wire["input"] {
		t.Fatalf("API input or compatible data alias lost: %s", encoded)
	}
}
