package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const (
	TokenIngestionPending  = "pending"
	TokenIngestionComplete = "complete"

	tokenBlockClaimLease = 15 * time.Minute
	tokenBlockBatchLimit = 256
	tokenBlockQueueTick  = 30 * time.Second
)

type tokenBlockWork struct {
	ID                   interface{}   `bson:"_id"`
	Result               models.Result `bson:"result"`
	BlockNumberInt       int64         `bson:"blockNumberInt"`
	IngestionState       string        `bson:"ingestionState"`
	TokenIngestionState  string        `bson:"tokenIngestionState"`
	TokenProcessingToken string        `bson:"tokenProcessingToken"`
	TokenAttempts        int           `bson:"tokenAttempts"`
}

func (work tokenBlockWork) block() models.ZondDatabaseBlock {
	return models.ZondDatabaseBlock{Result: work.Result}
}

// TokenSyncConfig holds configuration for token sync operations
type TokenSyncConfig struct {
	BatchSize       int
	BatchDelayMs    int
	QueryTimeoutSec int
}

// DefaultTokenSyncConfig returns the default token sync configuration
func DefaultTokenSyncConfig() TokenSyncConfig {
	return TokenSyncConfig{
		BatchSize:       10,
		BatchDelayMs:    86,
		QueryTimeoutSec: 30,
	}
}

// ProcessTokensAfterInitialSync handles token transfer processing after the initial block sync is complete.
// It queries blocks with transactions and processes token transfers in batches.
func ProcessTokensAfterInitialSync(initialSyncStart string, maxHex string) {
	config := DefaultTokenSyncConfig()
	processTokensWithConfig(initialSyncStart, maxHex, config)
}

// processTokensWithConfig processes tokens with the given configuration
func processTokensWithConfig(initialSyncStart string, maxHex string, config TokenSyncConfig) {
	configs.Logger.Info("Beginning post-sync token transfer processing",
		zap.String("from_block", initialSyncStart),
		zap.String("to_block", maxHex))

	// Get blocks with transactions only
	blocksWithTxs, err := getBlocksWithTransactions(initialSyncStart, maxHex, config.QueryTimeoutSec)
	if err != nil {
		configs.Logger.Error("Failed to query blocks with transactions", zap.Error(err))
		return
	}

	if len(blocksWithTxs) == 0 {
		configs.Logger.Info("No blocks with transactions found in range")
		return
	}

	configs.Logger.Info("Found blocks with transactions to process",
		zap.Int("count", len(blocksWithTxs)))

	// Process token transfers in batches
	processTokenTransferBatches(blocksWithTxs, config)
}

// getBlocksWithTransactions queries the database for blocks that have at least one
// transaction and fall within [fromBlock, toBlock] (inclusive).
// It uses the blockNumberInt field for numeric range comparison so that MongoDB's
// $gte/$lte operators work correctly regardless of hex string length.
func getBlocksWithTransactions(fromBlock, toBlock string, timeoutSec int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	fromInt := utils.HexToInt64(fromBlock)
	toInt := utils.HexToInt64(toBlock)

	filter := completedBlockRangeFilter(fromInt, toInt)

	projection := bson.M{"result.number": 1, "_id": 0}

	cursor, err := configs.BlocksCollections.Find(
		ctx,
		filter,
		options.Find().SetProjection(projection).SetSort(bson.D{{Key: "blockNumberInt", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var blocksWithTxs []string
	for cursor.Next(ctx) {
		var block struct {
			Result struct {
				Number string `bson:"number"`
			} `bson:"result"`
		}

		if err := cursor.Decode(&block); err != nil {
			return nil, fmt.Errorf("decode token block candidate: %w", err)
		}

		blocksWithTxs = append(blocksWithTxs, block.Result.Number)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return blocksWithTxs, nil
}

func completedBlockRangeFilter(fromInt, toInt int64) bson.M {
	return bson.M{
		"ingestionState":        db.BlockIngestionComplete,
		"result.transactions.0": bson.M{"$exists": true},
		"blockNumberInt": bson.M{
			"$gte": fromInt,
			"$lte": toInt,
		},
	}
}

// processTokenTransferBatches processes token transfers in configurable batches
func processTokenTransferBatches(blocksWithTxs []string, config TokenSyncConfig) {
	totalProcessed := 0
	batchCounter := 0

	for i := 0; i < len(blocksWithTxs); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(blocksWithTxs) {
			end = len(blocksWithTxs)
		}

		batchBlocks := blocksWithTxs[i:end]
		batchSize := len(batchBlocks)

		configs.Logger.Info("Processing token transfers batch",
			zap.Int("batch", batchCounter),
			zap.Int("size", batchSize))

		for _, blockNumber := range batchBlocks {
			if err := ProcessTokenTransfersForBlock(blockNumber); err != nil {
				configs.Logger.Error("Token block remains queued for retry",
					zap.String("blockNumber", blockNumber),
					zap.Error(err))
			}
			totalProcessed++
		}

		configs.Logger.Info("Completed token transfer batch",
			zap.Int("batch", batchCounter),
			zap.Int("blocks_processed", batchSize),
			zap.Int("total_processed", totalProcessed))

		batchCounter++

		// Add delay between batches to prevent overwhelming the node
		if config.BatchDelayMs > 0 {
			time.Sleep(time.Duration(config.BatchDelayMs) * time.Millisecond)
		}
	}

	configs.Logger.Info("Completed token transfer processing for all blocks with transactions",
		zap.Int("total_blocks_processed", totalProcessed))
}

func canonicalTokenQueueNumber(blockNumber string) (string, int64, error) {
	blockNumber = strings.ToLower(blockNumber)
	if !validation.IsValidHexString(blockNumber) {
		return "", 0, fmt.Errorf("invalid token block number: %s", blockNumber)
	}
	number := new(big.Int)
	if _, ok := number.SetString(strings.TrimPrefix(blockNumber, "0x"), 16); !ok {
		return "", 0, fmt.Errorf("invalid token block number: %s", blockNumber)
	}
	canonical := "0x" + number.Text(16)
	if !number.IsInt64() {
		return "", 0, fmt.Errorf("token block number exceeds int64: %s", blockNumber)
	}
	return canonical, number.Int64(), nil
}

func exactTokenIdentityRegex(value string) primitive.Regex {
	return primitive.Regex{
		Pattern: "^" + regexp.QuoteMeta(value) + "$",
		Options: "i",
	}
}

func loadUniqueCompleteTokenBlock(
	ctx context.Context,
	blockNumber string,
	blockHash string,
) (tokenBlockWork, error) {
	if configs.BlocksCollections == nil {
		return tokenBlockWork{}, fmt.Errorf("blocks collection is unavailable")
	}
	canonicalNumber, numberInt, err := canonicalTokenQueueNumber(blockNumber)
	if err != nil {
		return tokenBlockWork{}, err
	}
	filter := bson.M{
		"ingestionState": db.BlockIngestionComplete,
		"blockNumberInt": numberInt,
		"result.number":  exactTokenIdentityRegex(canonicalNumber),
	}
	if blockHash != "" {
		blockHash = strings.ToLower(blockHash)
		if err := validation.ValidateHexString(blockHash, validation.HashLength); err != nil {
			return tokenBlockWork{}, fmt.Errorf("invalid token block hash: %w", err)
		}
	}

	cursor, err := configs.BlocksCollections.Find(
		ctx,
		filter,
		options.Find().SetLimit(2),
	)
	if err != nil {
		return tokenBlockWork{}, fmt.Errorf("load complete token block %s: %w", canonicalNumber, err)
	}
	defer cursor.Close(ctx)
	var rows []tokenBlockWork
	if err := cursor.All(ctx, &rows); err != nil {
		return tokenBlockWork{}, fmt.Errorf("decode complete token block %s: %w", canonicalNumber, err)
	}
	if len(rows) == 0 {
		return tokenBlockWork{}, fmt.Errorf("complete token block %s %s is absent", canonicalNumber, blockHash)
	}
	if len(rows) != 1 {
		return tokenBlockWork{}, fmt.Errorf("complete token block %s %s is not unique", canonicalNumber, blockHash)
	}
	if blockHash != "" && !strings.EqualFold(rows[0].Result.Hash, blockHash) {
		return tokenBlockWork{}, fmt.Errorf(
			"complete token block %s hash %s does not match claimed hash %s",
			canonicalNumber,
			rows[0].Result.Hash,
			blockHash,
		)
	}
	return rows[0], nil
}

func tokenStateMissingFilter() bson.A {
	return bson.A{
		bson.M{"tokenIngestionState": bson.M{"$exists": false}},
		bson.M{"tokenIngestionState": ""},
	}
}

func initializeTokenBlockStates(ctx context.Context) error {
	if configs.BlocksCollections == nil {
		return fmt.Errorf("blocks collection is unavailable")
	}
	missing := tokenStateMissingFilter()
	emptyNeedsCompletion := append(tokenStateMissingFilter(), bson.M{
		"tokenIngestionState": TokenIngestionPending,
	})
	_, emptyErr := configs.BlocksCollections.UpdateMany(
		ctx,
		bson.M{
			"ingestionState": db.BlockIngestionComplete,
			"$or":            emptyNeedsCompletion,
			"result.transactions.0": bson.M{
				"$exists": false,
			},
		},
		bson.M{
			"$set":         bson.M{"tokenIngestionState": TokenIngestionComplete},
			"$currentDate": bson.M{"tokenCompletedAt": true},
		},
	)
	_, pendingErr := configs.BlocksCollections.UpdateMany(
		ctx,
		bson.M{
			"ingestionState":        db.BlockIngestionComplete,
			"$or":                   missing,
			"result.transactions.0": bson.M{"$exists": true},
		},
		bson.M{
			"$set": bson.M{"tokenIngestionState": TokenIngestionPending},
			"$unset": bson.M{
				"tokenProcessingToken":     "",
				"tokenProcessingStartedAt": "",
				"tokenProcessingUntil":     "",
				"tokenNextAttemptAt":       "",
				"lastTokenError":           "",
				"lastTokenFailedAt":        "",
				"tokenCompletedAt":         "",
			},
		},
	)
	stateErr := validateTokenBlockStates(ctx)
	return errors.Join(emptyErr, pendingErr, stateErr)
}

func invalidTokenBlockStateFilter() bson.M {
	return bson.M{
		"ingestionState": db.BlockIngestionComplete,
		"tokenIngestionState": bson.M{
			"$nin": bson.A{TokenIngestionPending, TokenIngestionComplete},
		},
	}
}

func validateTokenBlockStates(ctx context.Context) error {
	if configs.BlocksCollections == nil {
		return fmt.Errorf("blocks collection is unavailable")
	}
	var invalid struct {
		ID     interface{} `bson:"_id"`
		Result struct {
			Number string `bson:"number"`
		} `bson:"result"`
		State interface{} `bson:"tokenIngestionState"`
	}
	err := configs.BlocksCollections.FindOne(
		ctx,
		invalidTokenBlockStateFilter(),
		options.FindOne().
			SetProjection(bson.M{"result.number": 1, "tokenIngestionState": 1}).
			SetSort(bson.D{{Key: "blockNumberInt", Value: 1}, {Key: "_id", Value: 1}}),
	).Decode(&invalid)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("validate token ingestion states: %w", err)
	}
	return fmt.Errorf(
		"complete block %s (%v) has unsupported tokenIngestionState %v; expected %q or %q",
		invalid.Result.Number,
		invalid.ID,
		invalid.State,
		TokenIngestionPending,
		TokenIngestionComplete,
	)
}

func ensureTokenBlockState(ctx context.Context, blockNumber string) (tokenBlockWork, error) {
	work, err := loadUniqueCompleteTokenBlock(ctx, blockNumber, "")
	if err != nil {
		return tokenBlockWork{}, err
	}
	if len(work.Result.Transactions) == 0 {
		if work.TokenIngestionState == TokenIngestionComplete {
			return work, nil
		}
		if work.TokenIngestionState != "" && work.TokenIngestionState != TokenIngestionPending {
			return tokenBlockWork{}, fmt.Errorf(
				"block %s has invalid token ingestion state %q",
				work.Result.Number,
				work.TokenIngestionState,
			)
		}
		result, err := configs.BlocksCollections.UpdateOne(
			ctx,
			bson.M{
				"_id":            work.ID,
				"ingestionState": db.BlockIngestionComplete,
				"$or": append(tokenStateMissingFilter(), bson.M{
					"tokenIngestionState": TokenIngestionPending,
				}),
				"result.transactions.0": bson.M{"$exists": false},
			},
			bson.M{
				"$set":         bson.M{"tokenIngestionState": TokenIngestionComplete},
				"$currentDate": bson.M{"tokenCompletedAt": true},
			},
		)
		if err != nil {
			return tokenBlockWork{}, fmt.Errorf("complete empty token block %s: %w", work.Result.Number, err)
		}
		if result.MatchedCount != 1 {
			return tokenBlockWork{}, fmt.Errorf("empty token block changed while completing %s", work.Result.Number)
		}
		work.TokenIngestionState = TokenIngestionComplete
		return work, nil
	}

	switch work.TokenIngestionState {
	case TokenIngestionPending, TokenIngestionComplete:
		return work, nil
	case "":
	default:
		return tokenBlockWork{}, fmt.Errorf(
			"block %s has invalid token ingestion state %q",
			work.Result.Number,
			work.TokenIngestionState,
		)
	}

	state := TokenIngestionPending
	update := bson.M{"$set": bson.M{"tokenIngestionState": state}}
	result, err := configs.BlocksCollections.UpdateOne(
		ctx,
		bson.M{
			"_id":            work.ID,
			"ingestionState": db.BlockIngestionComplete,
			"$or":            tokenStateMissingFilter(),
		},
		update,
	)
	if err != nil {
		return tokenBlockWork{}, fmt.Errorf("initialize token state for block %s: %w", work.Result.Number, err)
	}
	if result.MatchedCount != 1 {
		return tokenBlockWork{}, fmt.Errorf("token state changed while initializing block %s", work.Result.Number)
	}
	work.TokenIngestionState = state
	return work, nil
}

func tokenBlockClaimFilter(blockNumber string) (bson.M, error) {
	epoch := time.Unix(0, 0).UTC()
	filter := bson.M{
		"ingestionState":        db.BlockIngestionComplete,
		"tokenIngestionState":   TokenIngestionPending,
		"result.transactions.0": bson.M{"$exists": true},
		"$expr": bson.M{"$and": bson.A{
			bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$tokenNextAttemptAt", epoch}},
				"$$NOW",
			}},
			bson.M{"$lte": bson.A{
				bson.M{"$ifNull": bson.A{"$tokenProcessingUntil", epoch}},
				"$$NOW",
			}},
		}},
	}
	if blockNumber != "" {
		canonicalNumber, numberInt, err := canonicalTokenQueueNumber(blockNumber)
		if err != nil {
			return nil, err
		}
		filter["blockNumberInt"] = numberInt
		filter["result.number"] = exactTokenIdentityRegex(canonicalNumber)
	}
	return filter, nil
}

func oldestPendingTokenBlockFilter() bson.M {
	return bson.M{
		"ingestionState":        db.BlockIngestionComplete,
		"tokenIngestionState":   TokenIngestionPending,
		"result.transactions.0": bson.M{"$exists": true},
	}
}

func incompleteTokenBlockFilter() bson.M {
	return bson.M{
		"ingestionState":      db.BlockIngestionComplete,
		"tokenIngestionState": bson.M{"$ne": TokenIngestionComplete},
	}
}

func loadOldestPendingTokenBlock(ctx context.Context) (tokenBlockWork, error) {
	if configs.BlocksCollections == nil {
		return tokenBlockWork{}, fmt.Errorf("blocks collection is unavailable")
	}
	var work tokenBlockWork
	err := configs.BlocksCollections.FindOne(
		ctx,
		oldestPendingTokenBlockFilter(),
		options.FindOne().SetSort(bson.D{
			{Key: "blockNumberInt", Value: 1},
			{Key: "_id", Value: 1},
		}),
	).Decode(&work)
	if err != nil {
		return tokenBlockWork{}, err
	}
	return work, nil
}

func claimOldestPendingTokenBlock(ctx context.Context) (tokenBlockWork, error) {
	oldest, err := loadOldestPendingTokenBlock(ctx)
	if err != nil {
		return tokenBlockWork{}, err
	}
	return claimTokenBlock(ctx, oldest.Result.Number)
}

func tokenBlockQueueHasIncompleteWork(ctx context.Context) (bool, error) {
	if configs.BlocksCollections == nil {
		return false, fmt.Errorf("blocks collection is unavailable")
	}
	err := configs.BlocksCollections.FindOne(
		ctx,
		incompleteTokenBlockFilter(),
		options.FindOne().SetProjection(bson.M{"_id": 1}),
	).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check incomplete token block queue: %w", err)
	}
	return true, nil
}

func tokenBlockClaimUpdate(token string) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"tokenProcessingToken":     token,
			"tokenProcessingStartedAt": "$$NOW",
			"tokenProcessingUntil": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(tokenBlockClaimLease / time.Second),
			}},
			"tokenAttempts": bson.M{"$add": bson.A{
				bson.M{"$ifNull": bson.A{"$tokenAttempts", 0}},
				1,
			}},
		}}},
	}
}

func claimTokenBlock(ctx context.Context, blockNumber string) (tokenBlockWork, error) {
	filter, err := tokenBlockClaimFilter(blockNumber)
	if err != nil {
		return tokenBlockWork{}, err
	}
	token := primitive.NewObjectID().Hex()
	var work tokenBlockWork
	err = configs.BlocksCollections.FindOneAndUpdate(
		ctx,
		filter,
		tokenBlockClaimUpdate(token),
		options.FindOneAndUpdate().
			SetSort(bson.D{{Key: "blockNumberInt", Value: 1}, {Key: "_id", Value: 1}}).
			SetReturnDocument(options.After),
	).Decode(&work)
	if err != nil {
		return tokenBlockWork{}, err
	}
	return work, nil
}

func tokenBlockOwnerFilter(work tokenBlockWork) bson.M {
	epoch := time.Unix(0, 0).UTC()
	return bson.M{
		"_id":                  work.ID,
		"ingestionState":       db.BlockIngestionComplete,
		"tokenIngestionState":  TokenIngestionPending,
		"tokenProcessingToken": work.TokenProcessingToken,
		"result.number":        exactTokenIdentityRegex(work.Result.Number),
		"result.hash":          exactTokenIdentityRegex(work.Result.Hash),
		"$expr": bson.M{"$gt": bson.A{
			bson.M{"$ifNull": bson.A{"$tokenProcessingUntil", epoch}},
			"$$NOW",
		}},
	}
}

func tokenBlockRenewalUpdate() mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"tokenProcessingUntil": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(tokenBlockClaimLease / time.Second),
			}},
		}}},
	}
}

func renewTokenBlockClaim(work tokenBlockWork) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := configs.BlocksCollections.UpdateOne(
		ctx,
		tokenBlockOwnerFilter(work),
		tokenBlockRenewalUpdate(),
	)
	if err != nil {
		return fmt.Errorf("renew token block claim: %w", err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("token block claim is expired or ownership was lost")
	}
	return nil
}

func tokenBlockRetryDelay(attempts int) time.Duration {
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

func tokenBlockFailureUpdate(message string, delay time.Duration) mongo.Pipeline {
	if len(message) > 2048 {
		message = message[:2048]
	}
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"lastTokenError":    message,
			"lastTokenFailedAt": "$$NOW",
			"tokenNextAttemptAt": bson.M{"$dateAdd": bson.M{
				"startDate": "$$NOW",
				"unit":      "second",
				"amount":    int64(delay / time.Second),
			}},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{
			"tokenProcessingToken",
			"tokenProcessingStartedAt",
			"tokenProcessingUntil",
		}}},
	}
}

func releaseFailedTokenBlockClaim(work tokenBlockWork, processingErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	delay := tokenBlockRetryDelay(work.TokenAttempts)
	result, err := configs.BlocksCollections.UpdateOne(
		ctx,
		tokenBlockOwnerFilter(work),
		tokenBlockFailureUpdate(processingErr.Error(), delay),
	)
	if err != nil {
		return errors.Join(processingErr, fmt.Errorf("release failed token block claim: %w", err))
	}
	if result.MatchedCount != 1 {
		return errors.Join(processingErr, fmt.Errorf("token block claim ownership was lost before failure release"))
	}
	return processingErr
}

func tokenBlockSnapshot(block models.ZondDatabaseBlock) ([]string, error) {
	number, _, err := canonicalTokenQueueNumber(block.Result.Number)
	if err != nil {
		return nil, err
	}
	hash := strings.ToLower(block.Result.Hash)
	if err := validation.ValidateHexString(hash, validation.HashLength); err != nil {
		return nil, err
	}
	timestamp := strings.ToLower(block.Result.Timestamp)
	if !validation.IsValidHexString(timestamp) {
		return nil, fmt.Errorf("invalid token block timestamp: %s", block.Result.Timestamp)
	}
	snapshot := []string{number, hash, timestamp, strings.ToLower(block.Result.TransactionsRoot)}
	for position, transaction := range block.Result.Transactions {
		txHash := strings.ToLower(transaction.Hash)
		if err := validation.ValidateHexString(txHash, validation.HashLength); err != nil {
			return nil, fmt.Errorf("transaction %d: %w", position, err)
		}
		txNumber, _, err := canonicalTokenQueueNumber(transaction.BlockNumber)
		if err != nil {
			return nil, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		txBlockHash := strings.ToLower(transaction.BlockHash)
		if err := validation.ValidateHexString(txBlockHash, validation.HashLength); err != nil {
			return nil, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		txIndex, _, err := canonicalTokenQueueNumber(transaction.TransactionIndex)
		if err != nil {
			return nil, fmt.Errorf("transaction %s: %w", txHash, err)
		}
		snapshot = append(snapshot, strings.Join([]string{txHash, txNumber, txBlockHash, txIndex}, "|"))
	}
	return snapshot, nil
}

func completeTokenBlockClaim(work tokenBlockWork, initialSnapshot []string) error {
	if err := renewTokenBlockClaim(work); err != nil {
		return fmt.Errorf("renew token block claim before completion recheck: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rechecked, err := loadUniqueCompleteTokenBlock(ctx, work.Result.Number, work.Result.Hash)
	if err != nil {
		return fmt.Errorf("recheck token block identity before completion: %w", err)
	}
	recheckedSnapshot, err := tokenBlockSnapshot(rechecked.block())
	if err != nil {
		return fmt.Errorf("recheck token block transaction set: %w", err)
	}
	if !reflect.DeepEqual(initialSnapshot, recheckedSnapshot) {
		return fmt.Errorf("token block identity changed before completion: %s %s",
			work.Result.Number, work.Result.Hash)
	}
	if err := renewTokenBlockClaim(work); err != nil {
		return fmt.Errorf("renew token block claim before completion update: %w", err)
	}
	result, err := configs.BlocksCollections.UpdateOne(
		ctx,
		tokenBlockOwnerFilter(work),
		bson.M{
			"$set":         bson.M{"tokenIngestionState": TokenIngestionComplete},
			"$currentDate": bson.M{"tokenCompletedAt": true},
			"$unset": bson.M{
				"tokenProcessingToken":     "",
				"tokenProcessingStartedAt": "",
				"tokenProcessingUntil":     "",
				"tokenNextAttemptAt":       "",
				"lastTokenError":           "",
				"lastTokenFailedAt":        "",
			},
		},
	)
	if err != nil {
		return fmt.Errorf("complete token block claim: %w", err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("token block claim ownership was lost before completion")
	}
	return nil
}

func processClaimedTokenBlock(work tokenBlockWork) error {
	if err := renewTokenBlockClaim(work); err != nil {
		return releaseFailedTokenBlockClaim(work, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	canonical, err := loadUniqueCompleteTokenBlock(ctx, work.Result.Number, work.Result.Hash)
	cancel()
	if err != nil {
		return releaseFailedTokenBlockClaim(work, err)
	}
	if !reflect.DeepEqual(canonical.ID, work.ID) {
		return releaseFailedTokenBlockClaim(work, fmt.Errorf(
			"token block claim does not own the unique canonical row %s %s",
			work.Result.Number,
			work.Result.Hash,
		))
	}
	initialSnapshot, err := tokenBlockSnapshot(canonical.block())
	if err != nil {
		return releaseFailedTokenBlockClaim(work, err)
	}
	if err := db.ProcessBlockTokenTransfersWithFence(canonical.block(), func() error {
		return renewTokenBlockClaim(work)
	}); err != nil {
		return releaseFailedTokenBlockClaim(work, err)
	}
	if err := completeTokenBlockClaim(work, initialSnapshot); err != nil {
		return releaseFailedTokenBlockClaim(work, err)
	}
	return nil
}

// ProcessTokenTransfersForBlock claims and processes one complete canonical
// block. Failures are returned and persisted with a server-time retry deadline.
func ProcessTokenTransfersForBlock(blockNumber string) error {
	lockChainMutation()
	defer chainMutationMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := validateTokenBlockStates(ctx); err != nil {
		cancel()
		return err
	}
	work, err := ensureTokenBlockState(ctx, blockNumber)
	cancel()
	if err != nil {
		return err
	}
	if work.TokenIngestionState == TokenIngestionComplete {
		return nil
	}
	oldestCtx, oldestCancel := context.WithTimeout(context.Background(), 30*time.Second)
	oldest, err := loadOldestPendingTokenBlock(oldestCtx)
	oldestCancel()
	if err != nil {
		return fmt.Errorf("load oldest pending token block: %w", err)
	}
	if !reflect.DeepEqual(oldest.ID, work.ID) {
		configs.Logger.Debug("Deferring token block behind an older pending block",
			zap.String("blockNumber", work.Result.Number),
			zap.String("oldestPendingBlock", oldest.Result.Number))
		return nil
	}

	claimCtx, claimCancel := context.WithTimeout(context.Background(), 30*time.Second)
	claimed, err := claimTokenBlock(claimCtx, blockNumber)
	claimCancel()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim token block %s: %w", blockNumber, err)
	}
	return processClaimedTokenBlock(claimed)
}

func pendingTokenBlockQueueStopped(stopCh <-chan struct{}) bool {
	if stopCh == nil {
		return false
	}
	select {
	case <-stopCh:
		return true
	default:
		return false
	}
}

func initializePendingTokenBlockQueue() error {
	lockChainMutation()
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	initErr := initializeTokenBlockStates(initCtx)
	initCancel()
	chainMutationMu.Unlock()
	if initErr != nil {
		return fmt.Errorf("initialize token block queue: %w", initErr)
	}
	return nil
}

func drainPendingTokenBlockClaims(stopCh <-chan struct{}) error {
	lockChainMutation()
	validationCtx, validationCancel := context.WithTimeout(context.Background(), 30*time.Second)
	validationErr := validateTokenBlockStates(validationCtx)
	validationCancel()
	chainMutationMu.Unlock()
	if validationErr != nil {
		return validationErr
	}
	var processingErrors []error
	for claimedCount := 0; claimedCount < tokenBlockBatchLimit; claimedCount++ {
		if pendingTokenBlockQueueStopped(stopCh) {
			break
		}
		lockChainMutation()
		claimCtx, claimCancel := context.WithTimeout(context.Background(), 30*time.Second)
		work, err := claimOldestPendingTokenBlock(claimCtx)
		claimCancel()
		if errors.Is(err, mongo.ErrNoDocuments) {
			chainMutationMu.Unlock()
			break
		}
		if err != nil {
			chainMutationMu.Unlock()
			processingErrors = append(processingErrors, fmt.Errorf("claim pending token block: %w", err))
			break
		}
		err = processClaimedTokenBlock(work)
		chainMutationMu.Unlock()
		if err != nil {
			processingErrors = append(processingErrors, err)
		}
	}
	return errors.Join(processingErrors...)
}

func tokenBlockQueueIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "ingestionState", Value: 1},
				{Key: "tokenIngestionState", Value: 1},
				{Key: "tokenNextAttemptAt", Value: 1},
				{Key: "tokenProcessingUntil", Value: 1},
				{Key: "blockNumberInt", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("token_ingestion_claim_idx"),
		},
		{
			Keys: bson.D{
				{Key: "ingestionState", Value: 1},
				{Key: "tokenIngestionState", Value: 1},
				{Key: "blockNumberInt", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("token_ingestion_oldest_idx"),
		},
	}
}

func initializeTokenBlockQueueIndexes() error {
	if configs.BlocksCollections == nil {
		return fmt.Errorf("blocks collection is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := configs.BlocksCollections.Indexes().CreateMany(ctx, tokenBlockQueueIndexModels()); err != nil {
		return fmt.Errorf("initialize token block queue indexes: %w", err)
	}
	return nil
}

func drainPendingTokenBlocks(stopCh <-chan struct{}) error {
	if err := initializePendingTokenBlockQueue(); err != nil {
		return err
	}
	return drainPendingTokenBlockClaims(stopCh)
}

// StartPendingTokenBlockWorker drains durable token block claims immediately
// at startup and on every quiet-head interval.
func StartPendingTokenBlockWorker(stopCh <-chan struct{}) {
	if !startPendingTokenBlockWorker(stopCh, tokenBlockQueueTick, func() error {
		return drainPendingTokenBlockClaims(stopCh)
	}) {
		configs.Logger.Warn("Pending token block worker was not started during shutdown")
	}
}

func startPendingTokenBlockWorker(
	stopCh <-chan struct{},
	interval time.Duration,
	process func() error,
) bool {
	return startBackgroundWorker(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		run := func() {
			if err := process(); err != nil {
				configs.Logger.Error("Pending token block processing completed with retryable failures",
					zap.Error(err))
			}
		}
		run()
		for {
			select {
			case <-ticker.C:
				run()
			case <-stopCh:
				return
			}
		}
	})
}

func completedBlockIdentityFilter(blockNumber string) bson.M {
	return bson.M{
		"result.number":  blockNumber,
		"ingestionState": db.BlockIngestionComplete,
	}
}

// InitializeTokenCollections initializes the token-related MongoDB collections.
// It attempts all initializations even if individual ones fail, collecting errors.
func InitializeTokenCollections() error {
	configs.Logger.Info("Initializing token collections")

	var initErrors []error
	if err := initializeTokenBlockQueueIndexes(); err != nil {
		configs.Logger.Error("Failed to initialize token block queue indexes", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token block queue indexes")
	}

	// Initialize token transfers collection
	if err := db.InitializeTokenTransfersCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token transfers collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token transfers collection")

		// Self-applying, idempotent backfill of blockNumberInt on legacy rows.
		// Runs synchronously AFTER the index exists so the numeric sorts (and
		// the backend, which sorts on the same field) order the full history
		// correctly before the sync loop starts. Bounded by batched bulk
		// writes; a second startup is a no-op once complete. A failure here is
		// non-fatal: log it and continue, the field still populates for new
		// rows at write time and the next restart retries the remaining rows.
		backfillCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		if err := db.BackfillTokenTransferBlockNumberInt(backfillCtx); err != nil {
			configs.Logger.Error("tokenTransfers blockNumberInt backfill failed; will retry on next restart",
				zap.Error(err))
		}
		cancel()
	}

	if err := db.InitializeTokenEventDeadLettersCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token event dead-letter collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token event dead-letter collection")
	}

	// Initialize token balances collection
	if err := db.InitializeTokenBalancesCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token balances collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token balances collection")
	}

	// Phase 3b: initialize per-tokenID metadata collection.
	if err := db.InitializeTokenMetadataCollection(); err != nil {
		configs.Logger.Error("Failed to initialize token metadata collection", zap.Error(err))
		initErrors = append(initErrors, err)
	} else {
		configs.Logger.Info("Successfully initialized token metadata collection")
	}

	// Return combined errors if any occurred
	if len(initErrors) > 0 {
		configs.Logger.Error("Token collection initialization completed with errors",
			zap.Int("error_count", len(initErrors)))
		return errors.Join(initErrors...)
	}

	return nil
}

// GetTokenSyncRange calculates the range of blocks to process for token sync
func GetTokenSyncRange(lastSyncedBlock string, maxHex string) (string, string) {
	initialSyncStart := db.GetLastKnownBlockNumberFromInitialSync()
	if initialSyncStart == "0x0" {
		initialSyncStart = "0x1"
	}

	// Verify we aren't trying to process tokens beyond what's actually synced
	if utils.CompareHexNumbers(lastSyncedBlock, "0x0") > 0 &&
		utils.CompareHexNumbers(maxHex, lastSyncedBlock) > 0 {
		maxHex = lastSyncedBlock
		configs.Logger.Info("Limiting token processing to last synced block",
			zap.String("lastSyncedBlock", lastSyncedBlock))
	}

	return initialSyncStart, maxHex
}

// StoreInitialSyncStartBlock stores the starting block number for the initial sync
func StoreInitialSyncStartBlock(blockNumber string) {
	err := db.StoreInitialSyncStartBlock(blockNumber)
	if err != nil {
		configs.Logger.Error("Failed to store initial sync start block",
			zap.String("block", blockNumber),
			zap.Error(err))
	}
}
