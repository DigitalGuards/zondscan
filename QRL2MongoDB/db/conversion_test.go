package db

import (
	"QRL2MongoDB/models"
	"strings"
	"testing"
)

func validReceiptFixture() (*models.ZondDatabaseBlock, *models.Transaction, *models.TransactionReceipt) {
	block := &models.ZondDatabaseBlock{Result: models.Result{
		Number: "0x2",
		Hash:   "0xblockhash",
	}}
	tx := &models.Transaction{Hash: "0xtxhash"}
	receipt := &models.TransactionReceipt{}
	receipt.Result.TransactionHash = tx.Hash
	receipt.Result.BlockNumber = block.Result.Number
	receipt.Result.BlockHash = block.Result.Hash
	receipt.Result.Status = "0x1"
	return block, tx, receipt
}

func TestValidateTransactionReceiptAcceptsMatchingMinedReceipt(t *testing.T) {
	block, tx, receipt := validReceiptFixture()
	if err := validateTransactionReceipt(block, tx, receipt); err != nil {
		t.Fatalf("validateTransactionReceipt() error = %v", err)
	}
}

func TestValidateTransactionReceiptRejectsUnavailableOrDivergentReceipt(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*models.TransactionReceipt)
		wantErr string
	}{
		{
			name:    "unavailable",
			mutate:  func(receipt *models.TransactionReceipt) { receipt.Result.TransactionHash = "" },
			wantErr: "not available yet",
		},
		{
			name:    "transaction hash",
			mutate:  func(receipt *models.TransactionReceipt) { receipt.Result.TransactionHash = "0xother" },
			wantErr: "mismatched receipt",
		},
		{
			name:    "block number",
			mutate:  func(receipt *models.TransactionReceipt) { receipt.Result.BlockNumber = "0x3" },
			wantErr: "reports block number 0x3",
		},
		{
			name:    "block hash",
			mutate:  func(receipt *models.TransactionReceipt) { receipt.Result.BlockHash = "0xother" },
			wantErr: "reports block hash 0xother",
		},
		{
			name:    "status",
			mutate:  func(receipt *models.TransactionReceipt) { receipt.Result.Status = "" },
			wantErr: "invalid status",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block, tx, receipt := validReceiptFixture()
			test.mutate(receipt)
			err := validateTransactionReceipt(block, tx, receipt)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
