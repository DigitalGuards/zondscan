package models

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestTransactionInputUsesCompatibleBSONDataField(t *testing.T) {
	var tx Transaction
	if err := json.Unmarshal([]byte(`{"input":"0x12345678","gas":"0x186a0"}`), &tx); err != nil {
		t.Fatal(err)
	}
	encoded, err := bson.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	var document bson.M
	if err := bson.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if document["data"] != "0x12345678" {
		t.Fatalf("persisted input=%v", document["data"])
	}
	if _, ok := document["input"]; ok {
		t.Fatal("changed legacy BSON calldata key")
	}
	legacy, err := bson.Marshal(bson.M{"data": "0xabcdef", "status": "0x0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := bson.Unmarshal(legacy, &tx); err != nil {
		t.Fatal(err)
	}
	if tx.Data != "0xabcdef" || tx.Status != "0x0" {
		t.Fatalf("legacy record changed: %+v", tx)
	}
}
