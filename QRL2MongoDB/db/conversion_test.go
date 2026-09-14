package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
	"strings"
	"testing"
)

func validReceiptFixture() (*models.ZondDatabaseBlock, *models.Transaction, *models.TransactionReceipt) {
	block := &models.ZondDatabaseBlock{Result: models.Result{
		Number: "0x2",
		Hash:   "0xblockhash",
	}}
	tx := &models.Transaction{Hash: "0xtxhash", Gas: "0x186a0", GasPrice: "0xff"}
	receipt := &models.TransactionReceipt{}
	receipt.Result.TransactionHash = tx.Hash
	receipt.Result.BlockNumber = block.Result.Number
	receipt.Result.BlockHash = block.Result.Hash
	receipt.Result.Status = "0x1"
	receipt.Result.GasUsed = "0x5208"
	receipt.Result.EffectiveGasPrice = "0x7"
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
		{name: "unknown status", mutate: func(r *models.TransactionReceipt) { r.Result.Status = "0x2" }, wantErr: "invalid status"},
		{name: "missing gas", mutate: func(r *models.TransactionReceipt) { r.Result.GasUsed = "" }, wantErr: "invalid gas used"},
		{name: "gas over limit", mutate: func(r *models.TransactionReceipt) { r.Result.GasUsed = "0x186a1" }, wantErr: "gas limit"},
		{name: "missing price", mutate: func(r *models.TransactionReceipt) { r.Result.EffectiveGasPrice = "" }, wantErr: "effective gas price"},
		{name: "malformed price", mutate: func(r *models.TransactionReceipt) { r.Result.EffectiveGasPrice = "0x-1" }, wantErr: "effective gas price"},
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

func TestReceiptAccountingSurvivesStorageWithoutAnotherRPC(t *testing.T) {
	block, tx, receipt := validReceiptFixture()
	block.Result.Transactions = []models.Transaction{*tx}
	receipt.Result.Status = "0x0"
	calls := 0
	if err := enrichTransactionReceipts(block, func(string) (*models.TransactionReceipt, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("temporary upstream failure")
		}
		return receipt, nil
	}); err != nil {
		t.Fatal(err)
	}
	encoded, err := bson.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	var stored models.ZondDatabaseBlock
	if err := bson.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	got := stored.Result.Transactions[0]
	fee, err := transactionReceiptFee(&got)
	if err != nil || fee.String() != "147000" || got.Status != "0x0" || calls != 1 {
		t.Fatalf("stored accounting = %+v, fee=%v, err=%v, calls=%d", got, fee, err, calls)
	}
}

func TestReceiptFailureRemainsRetryableAndClearsOldAccounting(t *testing.T) {
	previous := configs.Logger
	configs.Logger = zap.NewNop()
	t.Cleanup(func() { configs.Logger = previous })
	block, tx, _ := validReceiptFixture()
	tx.Status, tx.GasUsed, tx.EffectiveGasPrice = "0x1", "0x5208", "0x7"
	block.Result.Transactions = []models.Transaction{*tx}
	if err := enrichTransactionReceipts(block, func(string) (*models.TransactionReceipt, error) {
		return nil, errors.New("unavailable")
	}); err == nil {
		t.Fatal("expected retryable error")
	}
	got := block.Result.Transactions[0]
	if got.Status != "" || got.GasUsed != "" || got.EffectiveGasPrice != "" {
		t.Fatalf("retained stale accounting: %+v", got)
	}
	if err := processTransactionData(&got, "0x1", "", "", "0x1", false, "0x1"); err == nil {
		t.Fatal("companion processing accepted missing receipt")
	}
}

func TestReceiptFeePreservesZeroAndRequiresActualGas(t *testing.T) {
	for _, status := range []string{"0x0", "0x1"} {
		tx := &models.Transaction{Status: status, Gas: "0x186a0", GasPrice: "0xff", GasUsed: "0x5208", EffectiveGasPrice: "0x0"}
		fee, err := transactionReceiptFee(tx)
		if err != nil || fee.Sign() != 0 {
			t.Fatalf("zero price: fee=%v, err=%v", fee, err)
		}
		tx.GasUsed = ""
		if _, err := transactionReceiptFee(tx); err == nil {
			t.Fatal("gas limit became gas used")
		}
	}
}
