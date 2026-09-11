package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// UpdateTransactionStatuses updates transaction receipt statuses before the
// block and its companion rows cross the durable ingestion boundary.
func UpdateTransactionStatuses(block *models.ZondDatabaseBlock) error {
	if block == nil {
		return errors.New("cannot update transaction statuses for a nil block")
	}
	var errs []error
	for index := range block.Result.Transactions {
		tx := &block.Result.Transactions[index]
		receipt, err := rpc.GetTransactionReceipt(tx.Hash)
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
	if receipt.Result.Status == "" || !validation.IsValidHexString(receipt.Result.Status) {
		return fmt.Errorf("transaction %s receipt has invalid status %q",
			tx.Hash, receipt.Result.Status)
	}
	return nil
}
