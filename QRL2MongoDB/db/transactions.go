package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// ProcessTransactions processes transaction data and reports every companion
// write failure so the durable block-ingestion marker can remain pending.
func ProcessTransactions(block models.ZondDatabaseBlock) error {
	var errs []error
	for _, tx := range block.Result.Transactions {
		to, contractAddress, statusTx, isContract, contractErr := processContracts(&tx)
		if contractErr != nil {
			errs = append(errs, contractErr)
		}

		if err := processTransactionData(&tx, block.Result.Timestamp, to, contractAddress, statusTx, isContract, block.Result.Size); err != nil {
			errs = append(errs, err)
		}

		// Store contract addresses for later token processing
		// Only queue if this is actually a contract (new creation or interaction with existing contract)
		// This avoids queuing regular wallet addresses which would just be filtered out later
		if contractAddress != "" {
			// New contract creation - always queue
			if err := QueuePotentialTokenContract(contractAddress, &tx, block.Result.Timestamp); err != nil {
				errs = append(errs, err)
			}
		} else if isContract && to != "" {
			// Transaction to an existing contract - queue for token processing
			if err := QueuePotentialTokenContract(to, &tx, block.Result.Timestamp); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// QueuePotentialTokenContract stores a mapping of potential token contract addresses
// to be processed later in a batch
func QueuePotentialTokenContract(address string, tx *models.Transaction, blockTimestamp string) error {
	// Skip if the address is empty
	if address == "" {
		return nil
	}

	pending, err := newPendingTokenContractWork(address, tx, blockTimestamp)
	if err != nil {
		return err
	}

	// Use the pending contracts collection to store addresses
	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	if collection == nil {
		return fmt.Errorf("pending_token_contracts collection is unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the document to insert
	doc := bson.M{
		"contractAddress": pending.ContractAddress,
		"txHash":          pending.TxHash,
		"blockNumber":     pending.BlockNumber,
		"blockHash":       pending.BlockHash,
		"blockTimestamp":  pending.BlockTimestamp,
		"processed":       false,
	}

	// Use upsert to prevent duplicates
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"contractAddress": pending.ContractAddress,
		"txHash":          pending.TxHash,
	}

	_, err = collection.UpdateOne(ctx, filter, bson.M{
		"$set": doc,
		"$setOnInsert": bson.M{
			"createdAt": time.Now().UTC(),
			"attempts":  0,
		},
		"$currentDate": bson.M{"updatedAt": true},
		"$unset": bson.M{
			"processing":          "",
			"processingToken":     "",
			"processingStartedAt": "",
			"processingUntil":     "",
			"nextAttemptAt":       "",
			"lastError":           "",
			"lastFailedAt":        "",
			"processedAt":         "",
		},
	}, opts)
	if err != nil {
		configs.Logger.Error("Failed to queue potential token contract",
			zap.String("address", pending.ContractAddress),
			zap.String("txHash", pending.TxHash),
			zap.Error(err))
		return err
	} else {
		configs.Logger.Debug("Queued potential token contract for later processing",
			zap.String("address", pending.ContractAddress),
			zap.String("txHash", pending.TxHash),
			zap.String("blockNumber", pending.BlockNumber))
	}
	return nil
}

const (
	pendingTokenClaimLease = 15 * time.Minute
	pendingTokenBatchLimit = 256
)

type pendingTokenContractWork struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	ContractAddress string             `bson:"contractAddress"`
	TxHash          string             `bson:"txHash"`
	BlockNumber     string             `bson:"blockNumber"`
	BlockHash       string             `bson:"blockHash"`
	BlockTimestamp  string             `bson:"blockTimestamp"`
	ProcessingToken string             `bson:"processingToken"`
	Attempts        int                `bson:"attempts"`
}

func newPendingTokenContractWork(
	address string,
	tx *models.Transaction,
	blockTimestamp string,
) (pendingTokenContractWork, error) {
	if tx == nil {
		return pendingTokenContractWork{}, fmt.Errorf("queue potential token contract: transaction is nil")
	}
	work, err := normalizePendingTokenContractWork(pendingTokenContractWork{
		ContractAddress: address,
		TxHash:          tx.Hash,
		BlockNumber:     tx.BlockNumber,
		BlockHash:       tx.BlockHash,
		BlockTimestamp:  blockTimestamp,
	})
	if err != nil {
		return pendingTokenContractWork{}, fmt.Errorf("queue potential token contract: %w", err)
	}
	if work.BlockHash == "" {
		return pendingTokenContractWork{}, fmt.Errorf("queue potential token contract: block hash is required")
	}
	if work.BlockTimestamp == "" {
		return pendingTokenContractWork{}, fmt.Errorf("queue potential token contract: block timestamp is required")
	}
	return work, nil
}

func normalizePendingTokenContractWork(work pendingTokenContractWork) (pendingTokenContractWork, error) {
	work.ContractAddress = validation.ConvertToQAddress(work.ContractAddress)
	if !validation.IsValidAddress(work.ContractAddress) {
		return pendingTokenContractWork{}, fmt.Errorf("invalid contract address: %s", work.ContractAddress)
	}

	var err error
	work.TxHash, err = canonicalFixedHash(work.TxHash, "transaction hash")
	if err != nil {
		return pendingTokenContractWork{}, err
	}
	work.BlockNumber, err = canonicalHexQuantity(work.BlockNumber, "block number")
	if err != nil {
		return pendingTokenContractWork{}, err
	}
	if work.BlockHash != "" {
		work.BlockHash, err = canonicalFixedHash(work.BlockHash, "block hash")
		if err != nil {
			return pendingTokenContractWork{}, err
		}
	}
	if work.BlockTimestamp != "" {
		work.BlockTimestamp, err = canonicalHexQuantity(work.BlockTimestamp, "block timestamp")
		if err != nil {
			return pendingTokenContractWork{}, err
		}
	}
	return work, nil
}

func canonicalFixedHash(value, field string) (string, error) {
	value = strings.ToLower(value)
	if err := validation.ValidateHexString(value, validation.HashLength); err != nil {
		return "", fmt.Errorf("invalid %s: %w", field, err)
	}
	return value, nil
}

func canonicalHexQuantity(value, field string) (string, error) {
	value = strings.ToLower(value)
	if !validation.IsValidHexString(value) {
		return "", fmt.Errorf("invalid %s: %s", field, value)
	}
	number := new(big.Int)
	if _, ok := number.SetString(strings.TrimPrefix(value, "0x"), 16); !ok {
		return "", fmt.Errorf("invalid %s: %s", field, value)
	}
	return "0x" + number.Text(16), nil
}

func pendingTokenClaimFilter() bson.M {
	epoch := time.Unix(0, 0).UTC()
	return bson.M{
		"processed": false,
		"$expr": bson.M{"$and": bson.A{
			bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$nextAttemptAt", epoch}},
				"$$NOW",
			}},
			bson.M{"$or": bson.A{
				bson.M{"$ne": bson.A{
					bson.M{"$ifNull": bson.A{"$processing", false}},
					true,
				}},
				bson.M{"$lte": bson.A{
					bson.M{"$ifNull": bson.A{"$processingUntil", epoch}},
					"$$NOW",
				}},
			}},
		}},
	}
}

func pendingTokenClaimSort() bson.D {
	return bson.D{
		{Key: "nextAttemptAt", Value: 1},
		{Key: "processingUntil", Value: 1},
		{Key: "createdAt", Value: 1},
		{Key: "_id", Value: 1},
	}
}

func pendingTokenClaimUpdate(token string) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"processing":          true,
			"processingToken":     token,
			"processingStartedAt": "$$NOW",
			"processingUntil": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(pendingTokenClaimLease / time.Second),
			}},
			"attempts": bson.M{"$add": bson.A{
				bson.M{"$ifNull": bson.A{"$attempts", 0}},
				1,
			}},
		}}},
	}
}

func pendingTokenClaimOwnerFilter(work pendingTokenContractWork) bson.M {
	filter := bson.M{
		"processed":       false,
		"processing":      true,
		"processingToken": work.ProcessingToken,
	}
	if !work.ID.IsZero() {
		filter["_id"] = work.ID
	} else {
		filter["contractAddress"] = work.ContractAddress
		filter["txHash"] = work.TxHash
	}
	return filter
}

func pendingTokenRenewClaimUpdate() mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"processingUntil": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(pendingTokenClaimLease / time.Second),
			}},
		}}},
	}
}

func renewPendingTokenClaim(
	collection *mongo.Collection,
	work pendingTokenContractWork,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := collection.UpdateOne(
		ctx,
		pendingTokenClaimOwnerFilter(work),
		pendingTokenRenewClaimUpdate(),
	)
	if err != nil {
		return fmt.Errorf("renew token claim: %w", err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("renew token claim: ownership lost")
	}
	return nil
}

func pendingTokenRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 7 {
		shift = 7
	}
	delay := 5 * time.Second * time.Duration(1<<shift)
	if delay > 10*time.Minute {
		return 10 * time.Minute
	}
	return delay
}

func pendingTokenFailureUpdate(message string, retryDelay time.Duration) mongo.Pipeline {
	if len(message) > 2048 {
		message = message[:2048]
	}
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"processing":   false,
			"lastError":    message,
			"lastFailedAt": "$$NOW",
			"nextAttemptAt": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(retryDelay / time.Second),
			}},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{
			"processingToken",
			"processingStartedAt",
			"processingUntil",
		}}},
	}
}

func pendingTokenSuccessUpdate() bson.M {
	return bson.M{
		"$set": bson.M{
			"processed":  true,
			"processing": false,
		},
		"$currentDate": bson.M{"processedAt": true},
		"$unset": bson.M{
			"processingToken":     "",
			"processingStartedAt": "",
			"processingUntil":     "",
			"nextAttemptAt":       "",
			"lastError":           "",
			"lastFailedAt":        "",
		},
	}
}

// ProcessTokenTransfersFromTransactions validates legacy receipt queue entries
// after transaction ingestion. Each item uses a durable server-time claim.
// Transfer, metadata, and balance effects belong to the block-wide token queue.
// A crash or error leaves the receipt item reclaimable and idempotent.
func ProcessTokenTransfersFromTransactions(stopCh ...<-chan struct{}) error {
	configs.Logger.Info("Processing of queued token contracts")

	collection := configs.GetCollection(configs.DB, "pending_token_contracts")
	if collection == nil {
		return fmt.Errorf("pending_token_contracts collection is unavailable")
	}

	// Count unprocessed items for logging
	countCtx, countCancel := context.WithTimeout(context.Background(), 30*time.Second)
	count, err := collection.CountDocuments(countCtx, bson.M{"processed": false})
	countCancel()
	if err != nil {
		configs.Logger.Error("Failed to count pending token contracts", zap.Error(err))
		return err
	}

	configs.Logger.Info("Found pending token contracts to process", zap.Int64("count", count))
	if count == 0 {
		configs.Logger.Info("No pending token contracts to process")
		return nil
	}

	processed := 0
	failed := 0
	claimed := 0
	var batchErrors []error
	claimFilter := pendingTokenClaimFilter()
	findOneOpts := options.FindOneAndUpdate().
		SetSort(pendingTokenClaimSort()).
		SetReturnDocument(options.After)

	for claimed < pendingTokenBatchLimit {
		if pendingTokenQueueStopped(stopCh) {
			break
		}
		claimToken := primitive.NewObjectID().Hex()
		var pending pendingTokenContractWork

		claimCtx, claimCancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := collection.FindOneAndUpdate(
			claimCtx,
			claimFilter,
			pendingTokenClaimUpdate(claimToken),
			findOneOpts,
		).Decode(&pending)
		claimCancel()

		if err == mongo.ErrNoDocuments {
			// No more unprocessed items
			break
		}
		if err != nil {
			configs.Logger.Error("Failed to claim pending token contract", zap.Error(err))
			batchErrors = append(batchErrors, fmt.Errorf("claim pending token contract: %w", err))
			break
		}
		claimed++

		configs.Logger.Debug("Processing token contract",
			zap.String("address", pending.ContractAddress),
			zap.String("txHash", pending.TxHash),
			zap.String("blockNumber", pending.BlockNumber))

		processErr := renewPendingTokenClaim(collection, pending)
		if processErr == nil {
			processErr = processTokenContract(pending)
		}
		ownerFilter := pendingTokenClaimOwnerFilter(pending)
		if processErr != nil {
			failed++
			retryDelay := pendingTokenRetryDelay(pending.Attempts)
			failureCtx, failureCancel := context.WithTimeout(context.Background(), 30*time.Second)
			result, updateErr := collection.UpdateOne(
				failureCtx,
				ownerFilter,
				pendingTokenFailureUpdate(processErr.Error(), retryDelay),
			)
			failureCancel()
			if updateErr != nil {
				processErr = errors.Join(processErr, fmt.Errorf("release failed token claim: %w", updateErr))
			} else if result.MatchedCount != 1 {
				processErr = errors.Join(processErr, fmt.Errorf("token claim ownership lost before failure release"))
			}
			configs.Logger.Error("Queued token contract processing failed",
				zap.String("address", pending.ContractAddress),
				zap.String("txHash", pending.TxHash),
				zap.Duration("retryAfter", retryDelay),
				zap.Error(processErr))
			batchErrors = append(batchErrors, processErr)
			continue
		}

		completeCtx, completeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, completeErr := collection.UpdateOne(
			completeCtx,
			ownerFilter,
			pendingTokenSuccessUpdate(),
		)
		completeCancel()
		if completeErr != nil {
			failed++
			batchErrors = append(batchErrors, fmt.Errorf(
				"complete pending token contract %s %s: %w",
				pending.ContractAddress,
				pending.TxHash,
				completeErr,
			))
			continue
		}
		if result.MatchedCount != 1 {
			failed++
			batchErrors = append(batchErrors, fmt.Errorf(
				"complete pending token contract %s %s: claim ownership lost",
				pending.ContractAddress,
				pending.TxHash,
			))
			continue
		}
		processed++
	}

	configs.Logger.Info("Completed batch processing of token contracts",
		zap.Int("claimed", claimed),
		zap.Int("processed", processed),
		zap.Int("failed", failed),
		zap.Int("batchLimit", pendingTokenBatchLimit))
	return errors.Join(batchErrors...)
}

func pendingTokenQueueStopped(stopCh []<-chan struct{}) bool {
	if len(stopCh) == 0 || stopCh[0] == nil {
		return false
	}
	select {
	case <-stopCh[0]:
		return true
	default:
		return false
	}
}

func exactCaseInsensitiveRegex(value string) primitive.Regex {
	return primitive.Regex{
		Pattern: "^" + regexp.QuoteMeta(value) + "$",
		Options: "i",
	}
}

func pendingTokenCanonicalBlockFilter(
	work pendingTokenContractWork,
	receipt *models.TransactionReceipt,
) bson.M {
	blockHash := strings.ToLower(receipt.Result.BlockHash)
	// Nested Result and Transaction model fields have JSON tags only. Mongo's
	// BSON encoder therefore lowercases their Go field names, which makes the
	// stored transaction keys blocknumber and blockhash.
	return bson.M{
		"ingestionState": BlockIngestionComplete,
		"blockNumberInt": utils.HexToInt64(work.BlockNumber),
		"result.number":  exactCaseInsensitiveRegex(work.BlockNumber),
		"result.hash":    exactCaseInsensitiveRegex(blockHash),
		"result.transactions": bson.M{"$elemMatch": bson.M{
			"hash":        exactCaseInsensitiveRegex(work.TxHash),
			"blocknumber": exactCaseInsensitiveRegex(work.BlockNumber),
			"blockhash":   exactCaseInsensitiveRegex(blockHash),
		}},
	}
}

func validatePendingTokenReceiptIdentity(
	work pendingTokenContractWork,
	receipt *models.TransactionReceipt,
) error {
	if receipt == nil {
		return fmt.Errorf("transaction receipt is nil")
	}

	receiptTxHash, err := canonicalFixedHash(receipt.Result.TransactionHash, "receipt transaction hash")
	if err != nil {
		return err
	}
	if receiptTxHash != work.TxHash {
		return fmt.Errorf("receipt transaction hash %s does not match queued hash %s",
			receiptTxHash, work.TxHash)
	}
	receiptBlockNumber, err := canonicalHexQuantity(receipt.Result.BlockNumber, "receipt block number")
	if err != nil {
		return err
	}
	if receiptBlockNumber != work.BlockNumber {
		return fmt.Errorf("receipt block number %s does not match queued block %s",
			receiptBlockNumber, work.BlockNumber)
	}
	receiptBlockHash, err := canonicalFixedHash(receipt.Result.BlockHash, "receipt block hash")
	if err != nil {
		return err
	}
	if work.BlockHash != "" && receiptBlockHash != work.BlockHash {
		return fmt.Errorf("receipt block hash %s does not match queued block hash %s",
			receiptBlockHash, work.BlockHash)
	}
	if _, err := canonicalHexQuantity(receipt.Result.Status, "receipt status"); err != nil {
		return err
	}

	target := receipt.Result.To
	if receipt.Result.ContractAddress != "" &&
		!validation.IsZeroAddress(receipt.Result.ContractAddress) {
		target = receipt.Result.ContractAddress
	}
	target = validation.ConvertToQAddress(target)
	if !validation.IsValidAddress(target) {
		return fmt.Errorf("receipt has invalid target address: %s", target)
	}
	if target != work.ContractAddress {
		return fmt.Errorf("receipt target %s does not match queued contract %s",
			target, work.ContractAddress)
	}

	for index, log := range receipt.Result.Logs {
		if log.Removed {
			return fmt.Errorf("receipt log %d is marked removed", index)
		}
		logTxHash, err := canonicalFixedHash(log.TransactionHash, "receipt log transaction hash")
		if err != nil {
			return fmt.Errorf("receipt log %d: %w", index, err)
		}
		if logTxHash != work.TxHash {
			return fmt.Errorf("receipt log %d transaction hash %s does not match queued hash %s",
				index, logTxHash, work.TxHash)
		}
		logBlockNumber, err := canonicalHexQuantity(log.BlockNumber, "receipt log block number")
		if err != nil {
			return fmt.Errorf("receipt log %d: %w", index, err)
		}
		if logBlockNumber != work.BlockNumber {
			return fmt.Errorf("receipt log %d block number %s does not match queued block %s",
				index, logBlockNumber, work.BlockNumber)
		}
		logBlockHash, err := canonicalFixedHash(log.BlockHash, "receipt log block hash")
		if err != nil {
			return fmt.Errorf("receipt log %d: %w", index, err)
		}
		if logBlockHash != receiptBlockHash {
			return fmt.Errorf("receipt log %d block hash %s does not match receipt block hash %s",
				index, logBlockHash, receiptBlockHash)
		}
		logAddress := validation.ConvertToQAddress(log.Address)
		if !validation.IsValidAddress(logAddress) {
			return fmt.Errorf("receipt log %d has invalid emitter: %s", index, log.Address)
		}
		if !validation.IsValidHexString(log.LogIndex) {
			return fmt.Errorf("receipt log %d has invalid log index: %s", index, log.LogIndex)
		}
	}
	return nil
}

func validatePendingTokenCanonicalBlock(
	work pendingTokenContractWork,
	receipt *models.TransactionReceipt,
) error {
	if configs.BlocksCollections == nil {
		return fmt.Errorf("blocks collection is unavailable")
	}
	blockNumberInt := utils.HexToInt64(work.BlockNumber)
	if work.BlockNumber != GenesisBlockHex && blockNumberInt == 0 {
		return fmt.Errorf("queued block number exceeds the indexed int64 range: %s", work.BlockNumber)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type canonicalBlock struct {
		Result struct {
			Timestamp string `bson:"timestamp"`
		} `bson:"result"`
	}
	var canonicalBlocks []canonicalBlock
	cursor, err := configs.BlocksCollections.Find(
		ctx,
		pendingTokenCanonicalBlockFilter(work, receipt),
		options.Find().SetProjection(bson.M{"result.timestamp": 1}).SetLimit(2),
	)
	if err != nil {
		return fmt.Errorf("load complete canonical block %s: %w", work.BlockNumber, err)
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &canonicalBlocks); err != nil {
		return fmt.Errorf("decode complete canonical block %s: %w", work.BlockNumber, err)
	}
	if len(canonicalBlocks) == 0 {
		return fmt.Errorf("queued transaction %s is absent from a complete canonical block %s",
			work.TxHash, work.BlockNumber)
	}
	if len(canonicalBlocks) != 1 {
		return fmt.Errorf("queued transaction %s matches multiple complete canonical blocks at %s",
			work.TxHash, work.BlockNumber)
	}
	canonical := canonicalBlocks[0]
	if work.BlockTimestamp != "" {
		timestamp, err := canonicalHexQuantity(canonical.Result.Timestamp, "canonical block timestamp")
		if err != nil {
			return err
		}
		if timestamp != work.BlockTimestamp {
			return fmt.Errorf("canonical block timestamp %s does not match queued timestamp %s",
				timestamp, work.BlockTimestamp)
		}
	}
	return nil
}

// processTokenContract validates one legacy queue item against its exact receipt
// and complete canonical block row. Token classification, transfer, metadata,
// and balance side effects are owned exclusively by the durable block-wide
// token worker, which preserves strict block-height ordering across retries.
func processTokenContract(pending pendingTokenContractWork) error {
	work, err := normalizePendingTokenContractWork(pending)
	if err != nil {
		return err
	}
	txHash := work.TxHash

	configs.Logger.Debug("Validating legacy token queue identity",
		zap.String("targetAddress", work.ContractAddress),
		zap.String("txHash", txHash))

	receipt, err := rpc.GetTransactionReceipt(txHash)
	if err != nil {
		return fmt.Errorf("get transaction receipt %s: %w", txHash, err)
	}
	if err := validatePendingTokenReceiptIdentity(work, receipt); err != nil {
		return fmt.Errorf("validate transaction receipt %s: %w", txHash, err)
	}
	if err := validatePendingTokenCanonicalBlock(work, receipt); err != nil {
		return fmt.Errorf("validate queued canonical identity %s: %w", txHash, err)
	}
	return nil
}

func processTransactionData(tx *models.Transaction, blockTimestamp string, to string, contractAddress string, statusTx string, isContract bool, size string) error {
	feesBig, err := transactionReceiptFee(tx)
	if err != nil {
		return err
	}
	var errs []error
	from := tx.From
	txHash := tx.Hash
	blockNumber := tx.BlockNumber
	pk := tx.PublicKey
	signature := tx.Signature
	data := tx.Data
	nonce := tx.Nonce
	txType := tx.Type

	// Convert value to float64 for display. Guard against empty/short
	// values, slicing [2:] on those panics; treat too-short as zero.
	value := new(big.Int)
	if len(tx.Value) > 2 {
		if _, ok := value.SetString(tx.Value[2:], 16); !ok {
			value.SetInt64(0)
		}
	}
	divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
	bigIntAsFloat := new(big.Float).SetInt(value)
	resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
	valueFloat64, _ := resultBigFloat.Float64()

	// Per-role address upserts. The old map iteration passed the RECIPIENT's
	// isContract flag to both parties, so any EOA that ever sent a tx to a
	// contract got a sticky isContract=true addresses row (UpsertTransactions
	// never demotes true). Senders are always EOAs on Zond: contract-originated
	// value moves are internal txs handled separately, so the sender is
	// upserted with isContract=false and only the recipient carries the
	// computed flag.
	if from != "" {
		if err := RefreshAddressBalance(from, false); err != nil {
			configs.Logger.Warn("Failed to refresh sender balance", zap.String("address", from), zap.Error(err))
			errs = append(errs, err)
		}
	}
	if to != "" {
		if err := RefreshAddressBalance(to, isContract); err != nil {
			configs.Logger.Warn("Failed to refresh recipient balance", zap.String("address", to), zap.Error(err))
			errs = append(errs, err)
		}
	}
	// Contract creation (to == ""): register the freshly created contract's
	// addresses row immediately with isContract=true instead of waiting for
	// its first inbound transaction.
	if contractAddress != "" {
		if err := RefreshAddressBalance(contractAddress, true); err != nil {
			configs.Logger.Warn("Failed to refresh created contract balance", zap.String("address", contractAddress), zap.Error(err))
			errs = append(errs, err)
		}
	}

	trace := rpc.CallDebugTraceTransaction(tx.Hash)
	if trace.Err != nil {
		configs.Logger.Warn("Debug trace failed; internal calls for this tx will be missing",
			zap.String("txHash", txHash), zap.Error(trace.Err))
		errs = append(errs, fmt.Errorf("debug trace %s: %w", txHash, trace.Err))
	}
	// Persist the nested (depth >= 1) call frames: value moved by contract
	// code rather than by the outer transaction itself, e.g. an HTLC claim
	// paying out contract-held funds. The top-level frame is intentionally
	// not stored; it would only duplicate the transactionByAddress row.
	if err := StoreInternalCalls(trace.InternalCalls, txHash, blockTimestamp, blockNumber); err != nil {
		configs.Logger.Error("Failed to store internal calls",
			zap.String("txHash", txHash), zap.Error(err))
		errs = append(errs, fmt.Errorf("store internal calls for %s: %w", txHash, err))
	}

	// Register contracts created by nested CREATE/CREATE2 frames (factory
	// deployments). Only effective when debug tracing is enabled: with
	// ENABLE_DEBUG_TRACE unset the trace carries no internal calls and this
	// is a no-op, matching how traces are gated everywhere else.
	if err := storeInternalContractCreations(
		trace.InternalCalls,
		from,
		txHash,
		blockNumber,
		tx.BlockHash,
		tx.ChainID,
		statusTx,
	); err != nil {
		errs = append(errs, err)
	}

	// Legacy float fields, kept for backward-compatible numeric queries (e.g.
	// amount $gt 0). The exact, drift-free value travels alongside them as the
	// amountWei / paidFeesWei base-10 integer strings (value and feesBig).
	divisor = new(big.Float).SetFloat64(float64(utils.QUANTA))
	feesFloat := new(big.Float).SetInt(feesBig)
	feesResult := new(big.Float).Quo(feesFloat, divisor)
	fees, _ := feesResult.Float64()

	if _, err := TransactionByAddressCollection(blockTimestamp, txType, from, to, txHash, valueFloat64, fees, blockNumber, value.String(), feesBig.String(), tx.Status); err != nil {
		errs = append(errs, fmt.Errorf("store transactionByAddress row for %s: %w", txHash, err))
	}
	if _, err := TransferCollection(blockNumber, blockTimestamp, from, to, txHash, pk, signature, nonce, valueFloat64, data, contractAddress, statusTx, size, fees); err != nil {
		errs = append(errs, fmt.Errorf("store transfer row for %s: %w", txHash, err))
	}
	return errors.Join(errs...)
}

func TransferCollection(blockNumber string, blockTimestamp string, from string, to string, hash string, pk string, signature string, nonce string, value float64, data string, contractAddress string, status string, size string, paidFees float64) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	from = validation.ConvertToQAddress(from)
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}
	if contractAddress != "" {
		contractAddress = validation.ConvertToQAddress(contractAddress)
	}

	var doc bson.D

	baseDoc := bson.D{
		{Key: "blockNumber", Value: blockNumber},
		{Key: "blockTimestamp", Value: blockTimestamp},
		{Key: "from", Value: from},
		{Key: "txHash", Value: hash},
		{Key: "pk", Value: pk},
		{Key: "signature", Value: signature},
		{Key: "nonce", Value: nonce},
		{Key: "value", Value: value},
		{Key: "status", Value: status},
		{Key: "size", Value: size},
		{Key: "paidFees", Value: paidFees},
	}

	if contractAddress == "" {
		doc = append(baseDoc, bson.E{Key: "to", Value: to})
		if data != "" {
			doc = append(doc, bson.E{Key: "data", Value: data})
		}
	} else {
		doc = append(baseDoc, bson.E{Key: "contractAddress", Value: contractAddress})
		if data != "" {
			doc = append(doc, bson.E{Key: "data", Value: data})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.TransferCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

// StoreInternalCalls persists each flattened nested call frame of one
// transaction as an internalTransactionByAddress row. Shared by the live
// sync path (processTransactions) and the historical backfill command.
// A failed insert does not stop the remaining frames; all failures are
// joined into the returned error.
func StoreInternalCalls(calls []rpc.InternalCall, txHash string, blockTimestamp string, blockNumber string) error {
	var errs []error
	for _, call := range calls {
		_, err := InternalTransactionByAddressCollection(
			call.Type,
			strings.ToLower(call.Type),
			txHash,
			call.From,
			call.To,
			call.Input,
			call.Output,
			call.TraceAddress,
			call.Value,
			call.Gas,
			call.GasUsed,
			"",
			"0x0",
			blockTimestamp,
			blockNumber,
		)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func InternalTransactionByAddressCollection(transactionType string, callType string, hash string, from string, to string, input string, output string, traceAddress []int, value float64, gas string, gasUsed string, addressFunctionIdentifier string, amountFunctionIdentifier string, blockTimestamp string, blockNumber string) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	if from != "" {
		from = validation.ConvertToQAddress(from)
	}
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}
	if addressFunctionIdentifier != "" {
		addressFunctionIdentifier = validation.ConvertToQAddress(addressFunctionIdentifier)
	}

	// blockNumber (raw hex string) lets Rollback delete this row on a reorg via
	// the same $in companionFilter used for the other append-only collections.
	doc := bson.D{
		{Key: "type", Value: transactionType},
		{Key: "callType", Value: callType},
		{Key: "hash", Value: hash},
		{Key: "from", Value: from},
		{Key: "to", Value: to},
		{Key: "input", Value: input},
		{Key: "output", Value: output},
		{Key: "traceAddress", Value: traceAddress},
		{Key: "value", Value: value},
		{Key: "gas", Value: gas},
		{Key: "gasUsed", Value: gasUsed},
		{Key: "addressFunctionIdentifier", Value: addressFunctionIdentifier},
		{Key: "amountFunctionIdentifier", Value: amountFunctionIdentifier},
		{Key: "blockTimestamp", Value: blockTimestamp},
		{Key: "blockNumber", Value: blockNumber},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.InternalTransactionByAddressCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the internalTransactionByAddress collection:", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// TransactionByAddressCollection persists a per-address transaction row.
// amount/paidFees are the legacy float64 columns (kept for backward-compatible
// numeric queries); amountWei/paidFeesWei are the exact base-10 wei integer
// strings the API uses to render QRL without float64 precision drift.
func TransactionByAddressCollection(timeStamp string, txType string, from string, to string, hash string, amount float64, paidFees float64, blockNumber string, amountWei string, paidFeesWei string, receiptStatus string) (*mongo.InsertOneResult, error) {
	// Normalize addresses to canonical Q-prefix form
	from = validation.ConvertToQAddress(from)
	if to != "" {
		to = validation.ConvertToQAddress(to)
	}

	doc := bson.D{
		{Key: "txType", Value: txType},
		{Key: "from", Value: from},
		{Key: "to", Value: to},
		{Key: "txHash", Value: hash},
		{Key: "timeStamp", Value: timeStamp},
		{Key: "amount", Value: amount},
		{Key: "amountWei", Value: amountWei},
		{Key: "paidFees", Value: paidFees},
		{Key: "paidFeesWei", Value: paidFeesWei},
		{Key: "feeSource", Value: "receipt"},
		{Key: "receiptStatus", Value: receiptStatus},
		{Key: "blockNumber", Value: blockNumber},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.TransactionByAddressCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the transactionByAddress collection: ", zap.Error(err))
	}

	return result, err
}

// RefreshAddressBalance fetches the current canonical balance of one address
// and clears any rollback stale marker only after the Mongo write succeeds.
func RefreshAddressBalance(address string, isContract bool) error {
	responseBalance, err := rpc.GetBalance(address)
	if err != nil {
		return fmt.Errorf("get native balance for %s: %w", address, err)
	}

	getBalanceResult := new(big.Int)
	if responseBalance != "" && len(responseBalance) > 2 {
		if _, ok := getBalanceResult.SetString(responseBalance[2:], 16); !ok {
			return fmt.Errorf("invalid native balance response for %s: %q", address, responseBalance)
		}
	} else {
		return fmt.Errorf("invalid native balance response for %s: %q", address, responseBalance)
	}

	divisor := new(big.Float).SetFloat64(float64(utils.QUANTA))
	bigIntAsFloat := new(big.Float).SetInt(getBalanceResult)
	resultBigFloat := new(big.Float).Quo(bigIntAsFloat, divisor)
	resultFloat64, _ := resultBigFloat.Float64()

	_, err = UpsertTransactions(address, resultFloat64, isContract)
	return err
}

func UpsertTransactions(address string, value float64, isContract bool) (*mongo.UpdateResult, error) {
	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)
	filter := bson.D{{Key: "id", Value: address}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// If this is flagged as a contract, update with that information
	if isContract {
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "id", Value: address},
				{Key: "balance", Value: value},
				{Key: "isContract", Value: true}, // Always set to true if we know it's a contract
			}},
			{Key: "$unset", Value: bson.D{
				{Key: "balanceStale", Value: ""},
				{Key: "balanceStaleAt", Value: ""},
			}},
		}
		opts := options.Update().SetUpsert(true)
		result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
		}
		return result, err
	}

	// If not flagged as a contract, we need to check if it's already marked as a contract
	// to avoid overwriting that information
	var existingDoc struct {
		IsContract bool `bson:"isContract"`
	}

	err := configs.AddressesCollections.FindOne(ctx, filter).Decode(&existingDoc)
	if err == nil && existingDoc.IsContract {
		// It's already marked as a contract, so keep that information
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "id", Value: address},
				{Key: "balance", Value: value},
				// Don't update isContract field since we want to keep it as true
			}},
			{Key: "$unset", Value: bson.D{
				{Key: "balanceStale", Value: ""},
				{Key: "balanceStaleAt", Value: ""},
			}},
		}
		opts := options.Update().SetUpsert(true)
		result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
		}
		return result, err
	}

	// If it's not in our database or not marked as a contract, proceed with the regular update
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "id", Value: address},
			{Key: "balance", Value: value},
			{Key: "isContract", Value: isContract},
		}},
		{Key: "$unset", Value: bson.D{
			{Key: "balanceStale", Value: ""},
			{Key: "balanceStaleAt", Value: ""},
		}},
	}
	opts := options.Update().SetUpsert(true)
	result, err := configs.AddressesCollections.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Warn("Failed to update address collection: ", zap.Error(err))
	}
	return result, err
}

func GetContractByAddress(address string) *models.ContractInfo {
	address = validation.ConvertToQAddress(address)
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
}
