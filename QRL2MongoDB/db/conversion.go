package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"go.uber.org/zap"
)

// UpdateTransactionStatuses retains validated receipt accounting before the
// block and its companion rows cross the durable ingestion boundary.
func UpdateTransactionStatuses(block *models.ZondDatabaseBlock) error {
	return enrichTransactionReceipts(block, rpc.GetTransactionReceipt)
}

func enrichTransactionReceipts(block *models.ZondDatabaseBlock, getReceipt func(string) (*models.TransactionReceipt, error)) error {
	if block == nil {
		return errors.New("cannot update transaction statuses for a nil block")
	}
	var errs []error
	for index := range block.Result.Transactions {
		tx := &block.Result.Transactions[index]
		// Never retain enrichment from a failed retry or an untrusted block RPC.
		tx.Status, tx.GasUsed, tx.EffectiveGasPrice = "", "", ""
		receipt, err := getReceipt(tx.Hash)
		if err != nil {
			configs.Logger.Warn("Failed to get transaction receipt",
				zap.String("hash", tx.Hash),
				zap.Error(err))
			errs = append(errs, fmt.Errorf("get receipt status for %s: %w",
				tx.Hash, err))
			continue
		}
		if err := validateTransactionReceipt(block, tx, receipt); err != nil {
			errs = append(errs, err)
			continue
		}
		tx.Status = receipt.Result.Status
		tx.GasUsed = receipt.Result.GasUsed
		tx.EffectiveGasPrice = receipt.Result.EffectiveGasPrice
	}
	return errors.Join(errs...)
}

func validateTransactionReceipt(
	block *models.ZondDatabaseBlock,
	tx *models.Transaction,
	receipt *models.TransactionReceipt,
) error {
	if receipt == nil {
		return fmt.Errorf("transaction %s returned a nil receipt", tx.Hash)
	}
	if receipt.Result.TransactionHash == "" {
		return fmt.Errorf("transaction %s receipt is not available yet", tx.Hash)
	}
	if !strings.EqualFold(receipt.Result.TransactionHash, tx.Hash) {
		return fmt.Errorf("transaction %s received mismatched receipt %s",
			tx.Hash, receipt.Result.TransactionHash)
	}
	if receipt.Result.BlockNumber != block.Result.Number {
		return fmt.Errorf("transaction %s receipt reports block number %s, want %s",
			tx.Hash, receipt.Result.BlockNumber, block.Result.Number)
	}
	if !strings.EqualFold(receipt.Result.BlockHash, block.Result.Hash) {
		return fmt.Errorf("transaction %s receipt reports block hash %s, want %s",
			tx.Hash, receipt.Result.BlockHash, block.Result.Hash)
	}
	if receipt.Result.Status != "0x0" && receipt.Result.Status != "0x1" {
		return fmt.Errorf("transaction %s receipt has invalid status %q",
			tx.Hash, receipt.Result.Status)
	}
	gasUsed, err := receiptQuantity(receipt.Result.GasUsed)
	if err != nil {
		return fmt.Errorf("transaction %s receipt has invalid gas used: %w", tx.Hash, err)
	}
	gasLimit, err := receiptQuantity(tx.Gas)
	if err != nil || gasUsed.Cmp(gasLimit) > 0 {
		return fmt.Errorf("transaction %s receipt gas exceeds or lacks its transaction gas limit", tx.Hash)
	}
	if _, err := receiptQuantity(receipt.Result.EffectiveGasPrice); err != nil {
		return fmt.Errorf("transaction %s receipt has invalid effective gas price: %w", tx.Hash, err)
	}
	return nil
}

func receiptQuantity(value string) (*big.Int, error) {
	if !validation.IsValidHexString(value) {
		return nil, fmt.Errorf("invalid hex quantity %q", value)
	}
	number, ok := new(big.Int).SetString(value[2:], 16)
	if !ok || number.Sign() < 0 || number.BitLen() > 256 {
		return nil, fmt.Errorf("invalid receipt quantity %q", value)
	}
	return number, nil
}

// transactionReceiptFee fails before companion writes if enrichment is absent.
// Zero-priced receipts remain zero; gas limits and trace gas are never billed.
func transactionReceiptFee(tx *models.Transaction) (*big.Int, error) {
	if tx.Status != "0x0" && tx.Status != "0x1" {
		return nil, fmt.Errorf("transaction %s lacks validated receipt status", tx.Hash)
	}
	gasUsed, err := receiptQuantity(tx.GasUsed)
	if err != nil {
		return nil, fmt.Errorf("transaction %s lacks receipt gas used: %w", tx.Hash, err)
	}
	gasPrice, err := receiptQuantity(tx.EffectiveGasPrice)
	if err != nil {
		return nil, fmt.Errorf("transaction %s lacks receipt effective gas price: %w", tx.Hash, err)
	}
	return new(big.Int).Mul(gasUsed, gasPrice), nil
}
