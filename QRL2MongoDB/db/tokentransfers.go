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
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenTransfer atomically inserts or refreshes one canonical transfer
// identity. Refreshing an existing row repairs legacy rows that have no
// blockHash and rebinds a replayed transaction to the current canonical block.
func StoreTokenTransfer(transfer models.TokenTransfer) error {
	// Get explicit reference to the tokenTransfers collection
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	transfer.BlockNumber, err = canonicalHexQuantity(transfer.BlockNumber, "token transfer block number")
	if err != nil {
		return err
	}
	transfer.BlockHash, err = canonicalFixedHash(transfer.BlockHash, "token transfer block hash")
	if err != nil {
		return err
	}
	transfer.TxHash, err = canonicalFixedHash(transfer.TxHash, "token transfer transaction hash")
	if err != nil {
		return err
	}
	if transfer.TransferType == "event" {
		transfer.LogIndex, err = canonicalHexQuantity(transfer.LogIndex, "token transfer log index")
		if err != nil {
			return err
		}
	}

	// Additional validation and normalization before inserting
	if transfer.From == "" {
		transfer.From = configs.QRLZeroAddress // Normalize empty from address to zero address
	}

	if transfer.To == "" {
		transfer.To = configs.QRLZeroAddress // Normalize empty to address to zero address
	}

	// Normalize addresses to canonical Q-prefix form
	transfer.From = validation.ConvertToQAddress(transfer.From)
	transfer.To = validation.ConvertToQAddress(transfer.To)
	transfer.ContractAddress = validation.ConvertToQAddress(transfer.ContractAddress)
	if !validation.IsValidAddress(transfer.ContractAddress) {
		return fmt.Errorf("invalid token transfer contract address: %s", transfer.ContractAddress)
	}
	if !validation.IsValidAddress(transfer.From) {
		return fmt.Errorf("invalid token transfer sender address: %s", transfer.From)
	}
	if !validation.IsValidAddress(transfer.To) {
		return fmt.Errorf("invalid token transfer recipient address: %s", transfer.To)
	}

	blockNumber := new(big.Int)
	blockNumber.SetString(strings.TrimPrefix(transfer.BlockNumber, "0x"), 16)
	if !blockNumber.IsInt64() {
		return fmt.Errorf("token transfer block number exceeds int64: %s", transfer.BlockNumber)
	}
	// Populate the numeric block number so sort operations order correctly.
	transfer.BlockNumberInt = utils.HexToInt64(transfer.BlockNumber)

	// Debug-level log for per-record operations; Info is reserved for batch summaries.
	configs.Logger.Debug("Inserting token transfer document",
		zap.String("token", transfer.TokenSymbol),
		zap.String("from", transfer.From),
		zap.String("to", transfer.To),
		zap.String("txHash", transfer.TxHash))

	filter := tokenTransferIdentityFilter(
		transfer.TxHash,
		transfer.ContractAddress,
		transfer.LogIndex,
		transfer.TokenID,
	)
	if transfer.TransferType == "event" && transfer.LogIndex != "" {
		// Rows written before logIndex was introduced cannot be mapped safely
		// when one transaction emitted multiple Transfer events. Canonical replay
		// reconstructs every event from exact block logs, so remove only the
		// ambiguous event alias before writing its indexed replacement. Preserve
		// direct-calldata rows, whose empty logIndex is an intentional identity.
		legacyResult, legacyErr := collection.DeleteMany(ctx, bson.M{
			"txHash":          transfer.TxHash,
			"contractAddress": transfer.ContractAddress,
			"transferType":    bson.M{"$ne": "direct"},
			"$or": bson.A{
				bson.M{"logIndex": ""},
				bson.M{"logIndex": nil},
				bson.M{"logIndex": bson.M{"$exists": false}},
			},
		})
		if legacyErr != nil {
			return fmt.Errorf("remove ambiguous legacy token transfer alias: %w", legacyErr)
		}
		if legacyResult.DeletedCount > 0 {
			configs.Logger.Info("Removed ambiguous legacy token transfer alias before canonical replay",
				zap.String("txHash", transfer.TxHash),
				zap.String("contract", transfer.ContractAddress),
				zap.Int64("deleted", legacyResult.DeletedCount))
		}
	}
	result, err := collection.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": transfer},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		configs.Logger.Error("Failed to store token transfer",
			zap.String("txHash", transfer.TxHash),
			zap.String("token", transfer.TokenSymbol),
			zap.Error(err))
		return err
	}

	configs.Logger.Debug("Successfully stored token transfer in database",
		zap.String("token", transfer.TokenSymbol),
		zap.String("txHash", transfer.TxHash),
		zap.Int64("matched", result.MatchedCount),
		zap.Int64("upserted", result.UpsertedCount))
	return nil
}

func tokenTransferIdentityFilter(txHash, contractAddress, logIndex, tokenID string) bson.M {
	filter := bson.M{
		"txHash":          txHash,
		"contractAddress": contractAddress,
	}

	andClauses := []bson.M{}
	if logIndex == "" {
		andClauses = append(andClauses, bson.M{"$or": []bson.M{
			{"logIndex": ""},
			{"logIndex": bson.M{"$exists": false}},
		}})
	} else {
		filter["logIndex"] = logIndex
	}
	if tokenID == "" {
		andClauses = append(andClauses, bson.M{"$or": []bson.M{
			{"tokenID": ""},
			{"tokenID": bson.M{"$exists": false}},
		}})
	} else {
		filter["tokenID"] = tokenID
	}
	if len(andClauses) > 0 {
		filter["$and"] = andClauses
	}
	return filter
}

// TokenTransferExists checks if a specific token transfer is already
// persisted. The dedupe key is (txHash, contractAddress, logIndex, tokenID):
//
//   - logIndex was added in #88 so multi-transfer txs (a DEX swap that
//     emits 3 Transfer events) don't collide on (txHash, contract) alone.
//   - tokenID was added with NFT support: ERC-1155 TransferBatch logs
//     produce N rows per log (one per (id, value) tuple) that share the
//     same (txHash, contract, logIndex), only tokenID distinguishes them.
//
// Legacy rows pre-date the LogIndex and TokenID fields, so a missing-field
// match is needed for the empty-string sentinel paths (rows written by the
// long-dead direct-calldata path carry LogIndex="" with TokenID=""; pre-NFT
// rows have no tokenID at all).
func TokenTransferExists(txHash, contractAddress, logIndex, tokenID string) (bool, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := tokenTransferIdentityFilter(txHash, contractAddress, logIndex, tokenID)

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to check if token transfer exists",
			zap.String("txHash", txHash),
			zap.String("contractAddress", contractAddress),
			zap.String("logIndex", logIndex),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return false, err
	}

	if count > 1 {
		return false, fmt.Errorf(
			"token transfer identity matches %d rows: %s %s %s %s",
			count,
			txHash,
			contractAddress,
			logIndex,
			tokenID,
		)
	}
	return count == 1, nil
}

type canonicalTokenBlock struct {
	number           string
	hash             string
	timestamp        string
	transactionIndex map[string]string
	transactionSet   []string
}

type plannedTokenLog struct {
	standard string
	rows     []models.TokenTransfer
}

func canonicalTokenBlockIdentity(block models.ZondDatabaseBlock) (canonicalTokenBlock, error) {
	number, err := canonicalHexQuantity(block.Result.Number, "token block number")
	if err != nil {
		return canonicalTokenBlock{}, err
	}
	hash, err := canonicalFixedHash(block.Result.Hash, "token block hash")
	if err != nil {
		return canonicalTokenBlock{}, err
	}
	timestamp, err := canonicalHexQuantity(block.Result.Timestamp, "token block timestamp")
	if err != nil {
		return canonicalTokenBlock{}, err
	}

	identity := canonicalTokenBlock{
		number:           number,
		hash:             hash,
		timestamp:        timestamp,
		transactionIndex: make(map[string]string, len(block.Result.Transactions)),
		transactionSet:   make([]string, 0, len(block.Result.Transactions)),
	}
	seenIndexes := make(map[string]struct{}, len(block.Result.Transactions))
	for position, transaction := range block.Result.Transactions {
		txHash, err := canonicalFixedHash(transaction.Hash, "block transaction hash")
		if err != nil {
			return canonicalTokenBlock{}, fmt.Errorf("transaction %d: %w", position, err)
		}
		txBlockNumber, err := canonicalHexQuantity(transaction.BlockNumber, "transaction block number")
		if err != nil {
			return canonicalTokenBlock{}, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		if txBlockNumber != number {
			return canonicalTokenBlock{}, fmt.Errorf(
				"transaction %s block number %s does not match block %s",
				txHash,
				txBlockNumber,
				number,
			)
		}
		txBlockHash, err := canonicalFixedHash(transaction.BlockHash, "transaction block hash")
		if err != nil {
			return canonicalTokenBlock{}, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		if txBlockHash != hash {
			return canonicalTokenBlock{}, fmt.Errorf(
				"transaction %s block hash %s does not match block %s",
				txHash,
				txBlockHash,
				hash,
			)
		}
		txIndex, err := canonicalHexQuantity(transaction.TransactionIndex, "transaction index")
		if err != nil {
			return canonicalTokenBlock{}, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		expectedIndex := "0x" + new(big.Int).SetInt64(int64(position)).Text(16)
		if txIndex != expectedIndex {
			return canonicalTokenBlock{}, fmt.Errorf(
				"transaction %s index %s does not match position %s",
				txHash,
				txIndex,
				expectedIndex,
			)
		}
		if _, exists := identity.transactionIndex[txHash]; exists {
			return canonicalTokenBlock{}, fmt.Errorf("duplicate transaction hash in block: %s", txHash)
		}
		if _, exists := seenIndexes[txIndex]; exists {
			return canonicalTokenBlock{}, fmt.Errorf("duplicate transaction index in block: %s", txIndex)
		}
		seenIndexes[txIndex] = struct{}{}
		identity.transactionIndex[txHash] = txIndex
		identity.transactionSet = append(identity.transactionSet, txHash+"@"+txIndex)
	}
	return identity, nil
}

func normalizeTokenBlockLogs(
	logs []models.Log,
	identity canonicalTokenBlock,
) ([]models.Log, error) {
	normalized := make([]models.Log, len(logs))
	seenLogIndexes := make(map[string]struct{}, len(logs))
	for position, log := range logs {
		if log.Removed {
			return nil, fmt.Errorf("token log %d is marked removed", position)
		}
		blockNumber, err := canonicalHexQuantity(log.BlockNumber, "log block number")
		if err != nil {
			return nil, fmt.Errorf("token log %d: %w", position, err)
		}
		if blockNumber != identity.number {
			return nil, fmt.Errorf("token log %d block number %s does not match %s",
				position, blockNumber, identity.number)
		}
		blockHash, err := canonicalFixedHash(log.BlockHash, "log block hash")
		if err != nil {
			return nil, fmt.Errorf("token log %d: %w", position, err)
		}
		if blockHash != identity.hash {
			return nil, fmt.Errorf("token log %d block hash %s does not match %s",
				position, blockHash, identity.hash)
		}
		txHash, err := canonicalFixedHash(log.TransactionHash, "log transaction hash")
		if err != nil {
			return nil, fmt.Errorf("token log %d: %w", position, err)
		}
		expectedTxIndex, member := identity.transactionIndex[txHash]
		if !member {
			return nil, fmt.Errorf("token log %d transaction %s is absent from block %s",
				position, txHash, identity.number)
		}
		txIndex, err := canonicalHexQuantity(log.TransactionIndex, "log transaction index")
		if err != nil {
			return nil, fmt.Errorf("token log %d: %w", position, err)
		}
		if txIndex != expectedTxIndex {
			return nil, fmt.Errorf("token log %d transaction index %s does not match %s",
				position, txIndex, expectedTxIndex)
		}
		logIndex, err := canonicalHexQuantity(log.LogIndex, "log index")
		if err != nil {
			return nil, fmt.Errorf("token log %d: %w", position, err)
		}
		if _, exists := seenLogIndexes[logIndex]; exists {
			return nil, fmt.Errorf("duplicate token log index in block: %s", logIndex)
		}
		seenLogIndexes[logIndex] = struct{}{}

		emitter := validation.ConvertToQAddress(log.Address)
		if !validation.IsValidAddress(emitter) {
			return nil, fmt.Errorf("token log %d has invalid emitter: %s", position, log.Address)
		}
		if len(log.Topics) == 0 {
			return nil, fmt.Errorf("token log %d has no event topic", position)
		}
		for topicIndex, topic := range log.Topics {
			topic = strings.ToLower(topic)
			if err := validation.ValidateHexString(topic, validation.AddressLength); err != nil {
				return nil, fmt.Errorf("token log %d topic %d: %w", position, topicIndex, err)
			}
			log.Topics[topicIndex] = topic
		}
		topic0 := log.Topics[0]
		switch topic0 {
		case rpc.TransferEventSignature, rpc.TransferSingleEventSignature, rpc.TransferBatchEventSignature:
		default:
			return nil, fmt.Errorf("token log %d has unexpected event signature %s", position, topic0)
		}
		data := strings.ToLower(log.Data)
		if !validation.IsValidHexString(data) || (len(data)-2)%2 != 0 {
			return nil, fmt.Errorf("token log %d has invalid data: %s", position, log.Data)
		}

		log.Address = emitter
		log.BlockNumber = blockNumber
		log.BlockHash = blockHash
		log.TransactionHash = txHash
		log.TransactionIndex = txIndex
		log.LogIndex = logIndex
		log.Data = data
		normalized[position] = log
	}
	return normalized, nil
}

func planTokenBlockLogs(
	logs []models.Log,
	identity canonicalTokenBlock,
	processingFence func() error,
) ([]plannedTokenLog, []models.TokenEventDeadLetter, error) {
	return planTokenBlockLogsWithClassification(
		logs,
		identity,
		processingFence,
		prepareContractClassification,
		persistPreparedContractClassification,
		StoreTokenEventDeadLetter,
	)
}

func planTokenBlockLogsWithClassification(
	logs []models.Log,
	identity canonicalTokenBlock,
	processingFence func() error,
	prepare func(string, string, string, string) (preparedContractClassification, error),
	persist func(preparedContractClassification) error,
	storeDeadLetter func(models.TokenEventDeadLetter) error,
) ([]plannedTokenLog, []models.TokenEventDeadLetter, error) {
	classifications := make(map[string]preparedContractClassification)
	preparedToPersist := make([]preparedContractClassification, 0)
	plans := make([]plannedTokenLog, 0, len(logs))
	deadLetters := make([]models.TokenEventDeadLetter, 0)
	seenRows := make(map[string]struct{})
	for _, log := range logs {
		classified, exists := classifications[log.Address]
		if !exists {
			var err error
			classified, err = prepare(
				log.Address,
				identity.number,
				identity.hash,
				log.TransactionHash,
			)
			if err != nil {
				return nil, nil, err
			}
			classifications[log.Address] = classified
			if classified.persist {
				preparedToPersist = append(preparedToPersist, classified)
			}
		}
		if classified.standard == "" {
			continue
		}
		if classified.contract == nil {
			return nil, nil, fmt.Errorf("classified token contract %s has no metadata row", log.Address)
		}
		rows, err := decodeTransferLog(
			log,
			classified.contract,
			classified.standard,
			identity.number,
			identity.hash,
			identity.timestamp,
		)
		if err != nil {
			deadLetters = append(deadLetters, models.TokenEventDeadLetter{
				BlockNumber:   identity.number,
				BlockHash:     identity.hash,
				TxHash:        log.TransactionHash,
				LogIndex:      log.LogIndex,
				Emitter:       log.Address,
				Topic0:        log.Topics[0],
				TokenStandard: classified.standard,
				Reason: boundedTokenEventDeadLetterReason(fmt.Sprintf(
					"deterministic token event decode rejected: %v",
					err,
				)),
			})
			continue
		}
		for _, row := range rows {
			key := strings.Join([]string{row.TxHash, row.ContractAddress, row.LogIndex, row.TokenID}, "|")
			if _, exists := seenRows[key]; exists {
				return nil, nil, fmt.Errorf("duplicate decoded token transfer identity: %s", key)
			}
			seenRows[key] = struct{}{}
		}
		plans = append(plans, plannedTokenLog{standard: classified.standard, rows: rows})
	}
	for _, deadLetter := range deadLetters {
		if err := checkTokenProcessingFence(processingFence, "store token event dead-letter"); err != nil {
			return nil, nil, err
		}
		if err := storeDeadLetter(deadLetter); err != nil {
			return nil, nil, err
		}
	}
	for _, classification := range preparedToPersist {
		if err := checkTokenProcessingFence(processingFence, "persist contract classification"); err != nil {
			return nil, nil, err
		}
		if err := persist(classification); err != nil {
			return nil, nil, err
		}
	}
	return plans, deadLetters, nil
}

func checkTokenProcessingFence(processingFence func() error, phase string) error {
	if processingFence == nil {
		return nil
	}
	if err := processingFence(); err != nil {
		return fmt.Errorf("token processing fence before %s: %w", phase, err)
	}
	return nil
}

func refreshTokenTransferState(row models.TokenTransfer, standard string) error {
	var errs []error
	switch standard {
	case rpc.StandardERC20:
		if err := StoreTokenBalanceAtBlock(
			row.ContractAddress,
			row.From,
			row.Amount,
			row.BlockNumber,
			row.BlockHash,
		); err != nil {
			errs = append(errs, fmt.Errorf("refresh sender token balance: %w", err))
		}
		if err := StoreTokenBalanceAtBlock(
			row.ContractAddress,
			row.To,
			row.Amount,
			row.BlockNumber,
			row.BlockHash,
		); err != nil {
			errs = append(errs, fmt.Errorf("refresh recipient token balance: %w", err))
		}
	case rpc.StandardERC721:
		id, ok := new(big.Int).SetString(row.TokenID, 10)
		if !ok {
			return fmt.Errorf("invalid ERC-721 token ID: %s", row.TokenID)
		}
		if err := StoreERC721OwnershipAtBlock(
			row.ContractAddress,
			id,
			row.BlockNumber,
			row.BlockHash,
		); err != nil {
			errs = append(errs, fmt.Errorf("refresh ERC-721 ownership: %w", err))
		}
		if err := StubTokenMetadata(
			context.Background(),
			row.ContractAddress,
			row.TokenID,
			rpc.StandardERC721,
		); err != nil {
			errs = append(errs, fmt.Errorf("upsert ERC-721 metadata stub: %w", err))
		}
	case rpc.StandardERC1155:
		id, ok := new(big.Int).SetString(row.TokenID, 10)
		if !ok {
			return fmt.Errorf("invalid ERC-1155 token ID: %s", row.TokenID)
		}
		if err := StoreERC1155BalanceAtBlock(
			row.ContractAddress,
			row.From,
			id,
			row.BlockNumber,
			row.BlockHash,
		); err != nil {
			errs = append(errs, fmt.Errorf("refresh ERC-1155 sender balance: %w", err))
		}
		if err := StoreERC1155BalanceAtBlock(
			row.ContractAddress,
			row.To,
			id,
			row.BlockNumber,
			row.BlockHash,
		); err != nil {
			errs = append(errs, fmt.Errorf("refresh ERC-1155 recipient balance: %w", err))
		}
		if err := StubTokenMetadata(
			context.Background(),
			row.ContractAddress,
			row.TokenID,
			rpc.StandardERC1155,
		); err != nil {
			errs = append(errs, fmt.Errorf("upsert ERC-1155 metadata stub: %w", err))
		}
	default:
		return fmt.Errorf("unsupported token standard: %s", standard)
	}
	return errors.Join(errs...)
}

// ProcessBlockTokenTransfers processes every token event from one exact
// complete canonical block. All log identities and decodes finish before any
// transfer, balance, or metadata write starts. Every failure reaches the
// durable block retry state through the caller.
func ProcessBlockTokenTransfers(block models.ZondDatabaseBlock) error {
	return ProcessBlockTokenTransfersWithFence(block, nil)
}

// ProcessBlockTokenTransfersWithFence processes one exact block while
// renewing and checking the caller's durable ownership fence before every
// mutation phase. A nil fence retains the historical direct-call behavior.
func ProcessBlockTokenTransfersWithFence(
	block models.ZondDatabaseBlock,
	processingFence func() error,
) error {
	identity, err := canonicalTokenBlockIdentity(block)
	if err != nil {
		return err
	}
	sigs := []string{
		rpc.TransferEventSignature,       // ERC-20 + ERC-721
		rpc.TransferSingleEventSignature, // ERC-1155
		rpc.TransferBatchEventSignature,  // ERC-1155
	}

	configs.Logger.Info("Searching for token transfers",
		zap.String("blockNumber", identity.number),
		zap.String("blockHash", identity.hash),
		zap.Strings("eventSignatures", sigs))

	response, err := rpc.ZondGetBlockLogsForBlock(identity.number, identity.hash, sigs)
	if err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("qrl_getLogs returned a nil response for block %s %s", identity.number, identity.hash)
	}
	if response.Result == nil {
		return fmt.Errorf("qrl_getLogs returned a null result for block %s %s", identity.number, identity.hash)
	}
	logs, err := normalizeTokenBlockLogs(response.Result, identity)
	if err != nil {
		return err
	}
	if len(logs) == 0 {
		configs.Logger.Debug("No token transfer logs found in block",
			zap.String("blockNumber", identity.number))
		return nil
	}
	plans, deadLetters, err := planTokenBlockLogs(logs, identity, processingFence)
	if err != nil {
		return err
	}
	for _, deadLetter := range deadLetters {
		configs.Logger.Warn("Quarantined deterministic malformed token event",
			zap.String("blockNumber", deadLetter.BlockNumber),
			zap.String("blockHash", deadLetter.BlockHash),
			zap.String("txHash", deadLetter.TxHash),
			zap.String("logIndex", deadLetter.LogIndex),
			zap.String("contract", deadLetter.Emitter),
			zap.String("standard", deadLetter.TokenStandard),
			zap.String("reason", deadLetter.Reason))
	}

	stored := 0
	var processingErrors []error
	for _, plan := range plans {
		for _, row := range plan.rows {
			exists, err := TokenTransferExists(row.TxHash, row.ContractAddress, row.LogIndex, row.TokenID)
			if err != nil {
				processingErrors = append(processingErrors, fmt.Errorf(
					"check token transfer %s %s: %w",
					row.TxHash,
					row.LogIndex,
					err,
				))
				continue
			}
			if err := checkTokenProcessingFence(processingFence, "store token transfer"); err != nil {
				return errors.Join(errors.Join(processingErrors...), err)
			}
			if err := StoreTokenTransfer(row); err != nil {
				processingErrors = append(processingErrors, fmt.Errorf(
					"store token transfer %s %s: %w",
					row.TxHash,
					row.LogIndex,
					err,
				))
			} else if !exists {
				stored++
			}
			if err := checkTokenProcessingFence(processingFence, "refresh token state"); err != nil {
				return errors.Join(errors.Join(processingErrors...), err)
			}
			if err := refreshTokenTransferState(row, plan.standard); err != nil {
				processingErrors = append(processingErrors, fmt.Errorf(
					"refresh token state %s %s: %w",
					row.TxHash,
					row.LogIndex,
					err,
				))
			}
		}
	}

	configs.Logger.Info("Finished processing token transfers",
		zap.String("blockNumber", identity.number),
		zap.String("blockHash", identity.hash),
		zap.Int("transfersStored", stored),
		zap.Int("eventsQuarantined", len(deadLetters)),
		zap.Int("errors", len(processingErrors)))

	return errors.Join(processingErrors...)
}

// decodeTransferLog dispatches a transfer log to the correct per-standard
// decoder. The (standard, topic0, len(topics)) triple disambiguates the
// ERC-20/ERC-721 topic-0 collision without ever guessing, the contract's
// persisted classification (from supportsInterface) is the source of truth.
//
// Returns a non-nil error only on truly mismatched shapes (e.g. a 4-topic
// Transfer on a contract we classified as ERC-20). Decoder helpers
// surface their own errors for malformed data.
func decodeTransferLog(
	log models.Log,
	contract *models.ContractInfo,
	standard, blockNumber, blockHash, blockTimestamp string,
) ([]models.TokenTransfer, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	topic0 := log.Topics[0]

	base := models.TokenTransfer{
		ContractAddress: log.Address,
		BlockNumber:     blockNumber,
		BlockHash:       blockHash,
		TxHash:          log.TransactionHash,
		LogIndex:        log.LogIndex,
		Timestamp:       blockTimestamp,
		TokenSymbol:     contract.Symbol,
		TokenDecimals:   contract.Decimals,
		TokenName:       contract.Name,
		TransferType:    "event",
		TokenStandard:   standard,
	}

	switch {
	case standard == rpc.StandardERC20 && topic0 == rpc.TransferEventSignature && len(log.Topics) == 3:
		return decodeERC20TransferRow(log, base)
	case standard == rpc.StandardERC721 && topic0 == rpc.TransferEventSignature && len(log.Topics) == 4:
		return decodeERC721TransferRow(log, base)
	case standard == rpc.StandardERC1155 && topic0 == rpc.TransferSingleEventSignature && len(log.Topics) == 4:
		return decodeERC1155TransferSingleRow(log, base)
	case standard == rpc.StandardERC1155 && topic0 == rpc.TransferBatchEventSignature && len(log.Topics) == 4:
		return decodeERC1155TransferBatchRows(log, base)
	default:
		return nil, fmt.Errorf("standard/topic mismatch: standard=%s topic0=%s topicCount=%d",
			standard, topic0, len(log.Topics))
	}
}

// decodeERC20TransferRow produces a single TokenTransfer for an ERC-20
// Transfer log. Amount is left as the raw hex `log.Data` for backward
// compatibility with rows written before Phase 1 (the parsed amount from
// ParseTransferEvent is deliberately unused).
func decodeERC20TransferRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, _, err := rpc.ParseTransferEvent(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = log.Data
	return []models.TokenTransfer{base}, nil
}

// decodeERC721TransferRow produces a single TokenTransfer for an ERC-721
// Transfer log (4 topics, tokenID in topic[3], data empty).
// Amount is the canonical "1", every ERC-721 transfer moves exactly one
// token, so aggregate "transfers moved" queries stay consistent.
func decodeERC721TransferRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, tokenID, err := rpc.ParseERC721Transfer(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = "1"
	base.TokenID = tokenID.String()
	return []models.TokenTransfer{base}, nil
}

// decodeERC1155TransferSingleRow produces a single TokenTransfer for an
// ERC-1155 TransferSingle log. Amount is the decimal-string value (not hex)
// since the data field carries a proper uint256 that we can normalise.
func decodeERC1155TransferSingleRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, id, value, err := rpc.ParseERC1155TransferSingle(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = value.String()
	base.TokenID = id.String()
	return []models.TokenTransfer{base}, nil
}

// decodeERC1155TransferBatchRows fans a single TransferBatch log out to N
// rows, one per (id, value) tuple. The compound unique index
// (txHash, contract, logIndex, tokenID) keeps the rows distinct on replay.
func decodeERC1155TransferBatchRows(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, ids, values, err := rpc.ParseERC1155TransferBatch(log)
	if err != nil {
		return nil, err
	}
	fromQ := normalizeAddress(from)
	toQ := normalizeAddress(to)
	type aggregate struct {
		id    *big.Int
		value *big.Int
	}
	aggregates := make([]aggregate, 0, len(ids))
	positions := make(map[string]int, len(ids))
	for i := range ids {
		key := ids[i].String()
		if position, exists := positions[key]; exists {
			aggregates[position].value.Add(aggregates[position].value, values[i])
			if aggregates[position].value.BitLen() > 256 {
				return nil, fmt.Errorf("ERC-1155 batch value sum exceeds uint256 for token ID %s", key)
			}
			continue
		}
		positions[key] = len(aggregates)
		aggregates = append(aggregates, aggregate{
			id:    new(big.Int).Set(ids[i]),
			value: new(big.Int).Set(values[i]),
		})
	}
	out := make([]models.TokenTransfer, len(aggregates))
	for i, item := range aggregates {
		row := base
		row.From = fromQ
		row.To = toQ
		row.Amount = item.value.String()
		row.TokenID = item.id.String()
		out[i] = row
	}
	return out, nil
}

// normalizeAddress maps degenerate empty / bare-Q forms to the canonical
// zero address; passes valid Q-addresses through unchanged.
func normalizeAddress(addr string) string {
	if addr == "" || addr == "q" || addr == "Q" {
		return configs.QRLZeroAddress
	}
	return addr
}
