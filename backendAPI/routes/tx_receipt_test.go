package routes

import (
	"backendAPI/models"
	"context"
	"testing"
)

func TestIndexedRevertAndFeeSurviveReceiptOutage(t *testing.T) {
	query := models.Transfer{TxHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", Status: "0x0", GasLimit: "0x186a0", GasUsed: "0x5208", GasPrice: "0xff", EffectiveGasPrice: "0x7", Input: "0x12345678"}
	logs := enrichTransfer(context.Background(), &query,
		func(context.Context, string) receiptSummary { return receiptSummary{} },
		func(context.Context, string) string { t.Error("indexed calldata triggered RPC"); return "" },
	)
	if query.Status != "0x0" || query.GasUsed != "0x5208" || query.PaidFees != "0.000000000000147000" || query.Input != "0x12345678" || len(logs) != 0 {
		t.Fatalf("outage changed indexed receipt: %+v", query)
	}
}

func TestMissingLegacyReceiptRemainsUnavailable(t *testing.T) {
	query := models.Transfer{TxHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", GasLimit: "0x186a0", GasPrice: "0xff"}
	enrichTransfer(context.Background(), &query,
		func(context.Context, string) receiptSummary { return receiptSummary{} },
		func(context.Context, string) string { return "" },
	)
	if query.Status != "" || query.GasUsed != "" || query.PaidFees != "" || query.Input != "" {
		t.Fatalf("invented legacy receipt: %+v", query)
	}
}

func TestMatchingLiveReceiptEnrichesLegacyWithoutOverwritingStoredStatus(t *testing.T) {
	query := models.Transfer{TxHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", Status: "0x0", GasLimit: "0x186a0"}
	enrichTransfer(context.Background(), &query,
		func(context.Context, string) receiptSummary {
			return receiptSummary{TransactionHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", Status: "0x1", GasUsed: "0x5208", EffectiveGasPrice: "0x0"}
		},
		func(context.Context, string) string { return "0x12345678" },
	)
	if query.Status != "0x0" || query.PaidFees != "0.000000000000000000" || query.Input != "0x12345678" {
		t.Fatalf("legacy enrichment=%+v", query)
	}
}

func TestDivergentReceiptCannotChangeIndexedTransaction(t *testing.T) {
	query := models.Transfer{TxHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", GasLimit: "0x186a0", Input: "0x"}
	logs := enrichTransfer(context.Background(), &query,
		func(context.Context, string) receiptSummary {
			return receiptSummary{TransactionHash: "0xtx", BlockHash: "0xorphan", BlockNumber: "0x2", Status: "0x1", GasUsed: "0x5208", EffectiveGasPrice: "0x7", Logs: []receiptLog{{Data: "0x"}}}
		},
		func(context.Context, string) string { t.Error("known empty calldata triggered RPC"); return "" },
	)
	if query.Status != "" || query.GasUsed != "" || query.PaidFees != "" || len(logs) != 0 {
		t.Fatalf("accepted orphan receipt: %+v", query)
	}
}

func TestOverLimitLiveReceiptGasRemainsUnavailable(t *testing.T) {
	query := models.Transfer{TxHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", GasLimit: "0x5208", Input: "0x"}
	enrichTransfer(context.Background(), &query,
		func(context.Context, string) receiptSummary {
			return receiptSummary{TransactionHash: "0xtx", BlockHash: "0xblock", BlockNumber: "0x2", Status: "0x1", GasUsed: "0x5209", EffectiveGasPrice: "0x7"}
		},
		func(context.Context, string) string { return "" },
	)
	if query.GasUsed != "" || query.PaidFees != "" {
		t.Fatalf("accepted over-limit receipt: %+v", query)
	}
}
