package db

import (
	"backendAPI/models"
	"testing"
)

func TestTransferProjectionPreservesReceiptAndKnownEmptyInput(t *testing.T) {
	block := models.ZondDatabaseBlock{Result: models.Result{Number: "0x2", Hash: "0xblock"}}
	tx := models.Transaction{Hash: "0xtx", Gas: "0x186a0", GasUsed: "0x5208", EffectiveGasPrice: "0x7", Status: "0x0", Data: "0x"}
	got := transferFromIndexedTransaction(block, tx)
	if got.GasUsed != "0x5208" || got.GasLimit != "0x186a0" || got.Status != "0x0" || got.EffectiveGasPrice != "0x7" || got.Input != "0x" || got.BlockHash != block.Result.Hash {
		t.Fatalf("lost indexed receipt: %+v", got)
	}
	tx.GasUsed, tx.EffectiveGasPrice, tx.Status, tx.Data = "", "", "", ""
	got = transferFromIndexedTransaction(block, tx)
	if got.GasUsed != "" || got.Status != "" || got.EffectiveGasPrice != "" || got.Input != "" {
		t.Fatalf("invented legacy receipt: %+v", got)
	}
	if got.Value != "0x0" || got.Nonce != "0x0" || got.Size != "0x0" || got.GasPrice != "0x0" {
		t.Fatalf("changed legacy quantity defaults: %+v", got)
	}
}
