package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Collection name constants for consistency
const (
	// SyncStateCollection is the collection for tracking sync state
	SyncStateCollection = "sync_state"

	// InitialSyncStateCollection is the collection for tracking initial sync state
	InitialSyncStateCollection = "sync_initial_state"

	// BlocksCollection is the collection for storing blocks
	BlocksCollection = "blocks"

	// DBTimeout is the default timeout for database operations
	DBTimeout = 10 * time.Second

	// LastSyncedBlockID is the ID for the last synced block document
	LastSyncedBlockID = "last_synced_block"

	// InitialSyncStartID is the ID for the initial sync start block document
	InitialSyncStartID = "initial_sync_start"

	// BlockIngestionMigrationID attests that legacy companions passed the
	// offline canonical continuity and parent-link audit.
	BlockIngestionMigrationID = "durable_block_companions_v1"

	// GenesisBlockHex is the genesis block number in hex
	GenesisBlockHex = "0x0"
)

// GetLatestBlockFromDB returns the latest block from the database
// Returns nil if no blocks exist or if there's an error
func GetLatestBlockFromDB() *models.ZondDatabaseBlock {
	if !IsCollectionsExist() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	// Sort by blockNumberInt (numeric), hex strings on result.number /
	// result.timestamp lex-sort wrong at width boundaries.
	findOptions := options.FindOne().
		SetProjection(bson.M{
			"result.number":    1,
			"result.hash":      1,
			"result.timestamp": 1,
		}).
		SetSort(bson.M{"blockNumberInt": -1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.M{
		"ingestionState": BlockIngestionComplete,
	}, findOptions).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to find latest block", zap.Error(err))
		return nil
	}

	return &block
}

// GetBlockFromDB retrieves a block by its number from the database
func GetBlockFromDB(blockNumber string) *models.ZondDatabaseBlock {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.M{"result.number": blockNumber}).Decode(&block)
	if err != nil {
		configs.Logger.Debug("Failed to find block", zap.String("blockNumber", blockNumber), zap.Error(err))
		return nil
	}

	return &block
}

// GetLatestBlockNumberFromDB returns the latest block number from the database
// Returns "0x0" if no blocks exist or if there's an error
func GetLatestBlockNumberFromDB() string {
	// Try to get the latest block
	block := GetLatestBlockFromDB()
	if block != nil && block.Result.Number != "" {
		return block.Result.Number
	}

	// If that fails, return "0x0"
	return GenesisBlockHex
}

// GetLatestBlockHashHeaderFromDB returns the hash of a block with the given number
// Returns empty string if the block doesn't exist or if there's an error
func GetLatestBlockHashHeaderFromDB(blockNumber string) string {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	// Query for the block by number, only retrieving the hash field
	findOptions := options.FindOne().SetProjection(bson.M{"result.hash": 1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(
		ctx,
		bson.M{"result.number": blockNumber},
		findOptions,
	).Decode(&block)

	if err != nil {
		configs.Logger.Info("Failed to find block hash",
			zap.String("block", blockNumber),
			zap.Error(err))
		return ""
	}

	return block.Result.Hash
}

// GetLastKnownBlockNumber retrieves the last known block number from the sync state
// Returns "0x0" if no sync state exists or if there's an error
func GetLastKnownBlockNumber() string {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	syncColl := configs.GetCollection(configs.DB, SyncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{
		"_id": LastSyncedBlockID,
	}).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("No sync state found, this appears to be the first run")
		} else {
			configs.Logger.Warn("Failed to get last known block number", zap.Error(err))
		}
		return GenesisBlockHex
	}

	if result.BlockNumber == "" {
		configs.Logger.Warn("Found sync state but block number is empty")
		return GenesisBlockHex
	}

	configs.Logger.Info("Found last known block in sync state",
		zap.String("block", result.BlockNumber))
	return result.BlockNumber
}

// GetLastKnownBlockNumberStrict returns database and schema errors to offline
// audits instead of converting them to a genesis cursor.
func GetLastKnownBlockNumberStrict() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	var result struct {
		BlockNumber string `bson:"block_number"`
	}
	syncColl := configs.GetCollection(configs.DB, SyncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{"_id": LastSyncedBlockID}).Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", ErrSyncStateNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read durable sync state: %w", err)
	}
	if result.BlockNumber == "" {
		return "", fmt.Errorf("durable sync state has an empty block number")
	}
	if _, err := parseStoredBlockNumber(result.BlockNumber); err != nil {
		return "", fmt.Errorf("durable sync state: %w", err)
	}
	return result.BlockNumber, nil
}

// StoreLastKnownBlockNumber updates the sync state with the given block number
// Only updates if the new block number is higher than the existing one
func StoreLastKnownBlockNumber(blockNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, SyncStateCollection)

	// First check if the document exists
	var existingDoc struct {
		BlockNumber string `bson:"block_number"`
	}

	err := syncColl.FindOne(ctx, bson.M{"_id": LastSyncedBlockID}).Decode(&existingDoc)

	blockNumberIntVal := utils.HexToInt64(blockNumber)

	if err == mongo.ErrNoDocuments {
		// Document doesn't exist, create it
		_, err = syncColl.InsertOne(ctx, bson.M{
			"_id":              LastSyncedBlockID,
			"block_number":     blockNumber,
			"block_number_int": blockNumberIntVal,
		})

		if err != nil {
			// If we get a duplicate key error, someone else created it first
			// Just log and continue to the update step
			if !strings.Contains(err.Error(), "E11000 duplicate key error") {
				configs.Logger.Warn("Failed to create sync state document",
					zap.String("block", blockNumber),
					zap.Error(err))
				return err
			}
		} else {
			configs.Logger.Info("Created new sync state document",
				zap.String("block", blockNumber))
			return nil
		}
	} else if err != nil {
		configs.Logger.Error("Error checking sync state document",
			zap.String("block", blockNumber),
			zap.Error(err))
		return err
	} else if existingDoc.BlockNumber != "" {
		if utils.CompareHexNumbers(existingDoc.BlockNumber, blockNumber) >= 0 {
			return nil
		}
	}

	// Document exists or was just created by another goroutine.
	// Use the integer field for the $lt guard so the comparison is numeric,
	// not the lexicographic hex string comparison that would produce wrong
	// ordering (e.g. "0x9" sorts after "0x10" lexicographically).
	//
	// Also accept docs that don't yet have block_number_int, legacy rows
	// written by an older syncer were missing the field, which made the
	// $lt match nothing and silently froze sync_state. The early-return
	// guard above (existing.BlockNumber >= new) already prevents
	// going-backwards for those rows.
	result, err := syncColl.UpdateOne(
		ctx,
		bson.M{
			"_id": LastSyncedBlockID,
			"$or": []bson.M{
				{"block_number_int": bson.M{"$lt": blockNumberIntVal}},
				{"block_number_int": bson.M{"$exists": false}},
			},
		},
		bson.M{"$set": bson.M{
			"block_number":     blockNumber,
			"block_number_int": blockNumberIntVal,
		}},
	)

	if err != nil {
		configs.Logger.Warn("Failed to update sync state",
			zap.String("block", blockNumber),
			zap.Error(err))
		return err
	}

	if result.ModifiedCount > 0 {
		configs.Logger.Info("Updated last synced block",
			zap.String("block", blockNumber))
	} else {
		configs.Logger.Debug("No update needed for sync state",
			zap.String("block", blockNumber))
	}

	return nil
}

// GetLastKnownBlockNumberFromInitialSync retrieves the first block number that was processed
// during the initial sync. Used for token transfer processing after initial sync.
func GetLastKnownBlockNumberFromInitialSync() string {
	// If we have a record of the first synced block, use that
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	syncColl := configs.GetCollection(configs.DB, InitialSyncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{
		"_id": InitialSyncStartID,
	}).Decode(&result)

	if err == nil && result.BlockNumber != "" {
		configs.Logger.Info("Found initial sync start block",
			zap.String("block", result.BlockNumber))
		return result.BlockNumber
	}

	// If no record exists, find the oldest block in the DB. Sort ascending by
	// blockNumberInt, sorting by result.number (hex string) would lex-sort
	// "0x10" before "0x9" and return the wrong "oldest" block.
	var block models.ZondDatabaseBlock
	findOptions := options.FindOne().SetProjection(bson.M{"result.number": 1}).SetSort(bson.M{"blockNumberInt": 1})
	err = configs.BlocksCollections.FindOne(ctx, bson.M{}, findOptions).Decode(&block)

	if err == nil && block.Result.Number != "" {
		// Store this for future reference
		_, _ = syncColl.UpdateOne(
			ctx,
			bson.M{"_id": InitialSyncStartID},
			bson.M{"$set": bson.M{"block_number": block.Result.Number}},
			options.Update().SetUpsert(true),
		)

		configs.Logger.Info("Using oldest block in DB as initial sync start",
			zap.String("block", block.Result.Number))
		return block.Result.Number
	}

	// If all else fails, start from genesis
	configs.Logger.Info("No initial sync start point found, starting from genesis")
	return GenesisBlockHex
}

// StoreInitialSyncStartBlock stores the block number that was used as the starting point
// for the initial sync. This is used for token transfer processing after initial sync.
func StoreInitialSyncStartBlock(blockNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, InitialSyncStateCollection)

	// Update or insert the initial sync start block
	_, err := syncColl.UpdateOne(
		ctx,
		bson.M{"_id": InitialSyncStartID},
		bson.M{"$set": bson.M{"block_number": blockNumber}},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		configs.Logger.Error("Failed to store initial sync start block",
			zap.String("block", blockNumber),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Stored initial sync start block",
		zap.String("block", blockNumber))
	return nil
}

var (
	ErrBlockWriteUnresolved   = errors.New("block batch write outcome is unresolved")
	ErrBlockHeightConflict    = errors.New("block height has multiple stored identities")
	ErrCanonicalBlockNotFound = errors.New("canonical block is missing")
	ErrBlockMigrationRequired = errors.New("legacy block ingestion migration is required")
	ErrSyncStateNotFound      = errors.New("durable sync state is missing")
)

const (
	BlockIngestionPending  = "pending"
	BlockIngestionComplete = "complete"
	BlockHeightUniqueIndex = "blockNumberInt_desc_idx"
)

// ConfirmedBlockWrite is a block whose exact height and hash were observed
// after the write attempt. AlreadyExisted reports that the same identity was
// present before this attempt, so callers can avoid duplicate companion rows.
type ConfirmedBlockWrite struct {
	Block              models.ZondDatabaseBlock
	AlreadyExisted     bool
	CompanionsComplete bool
}

// BlockBatchWriteResult contains the contiguous canonical prefix confirmed by
// a post-write read. Attempted counts the sorted, de-duplicated input blocks.
type BlockBatchWriteResult struct {
	Confirmed []ConfirmedBlockWrite
	Attempted int
}

type blockWriteCandidate struct {
	block     models.ZondDatabaseBlock
	numberInt int64
}

type storedBlockState struct {
	hash           string
	ingestionState string
}

// BlockHeightConflictError identifies duplicate or conflicting rows at one
// requested height.
type BlockHeightConflictError struct {
	Number   string
	Expected string
	Found    []string
}

func (e *BlockHeightConflictError) Error() string {
	return fmt.Sprintf("%v: height %s expected %s, found %v",
		ErrBlockHeightConflict, e.Number, e.Expected, e.Found)
}

func (e *BlockHeightConflictError) Unwrap() error {
	return ErrBlockHeightConflict
}

func parseStoredBlockNumber(blockNumber string) (int64, error) {
	if !strings.HasPrefix(blockNumber, "0x") || len(blockNumber) == 2 {
		return 0, fmt.Errorf("invalid block number %q", blockNumber)
	}
	value, err := strconv.ParseUint(blockNumber[2:], 16, 63)
	if err != nil {
		return 0, fmt.Errorf("invalid block number %q: %w", blockNumber, err)
	}
	return int64(value), nil
}

func canonicalStoredBlockNumber(number int64) string {
	return "0x" + strconv.FormatInt(number, 16)
}

func sortedBlockWriteCandidates(blocks []models.ZondDatabaseBlock) ([]blockWriteCandidate, error) {
	candidates := make([]blockWriteCandidate, 0, len(blocks))
	seen := make(map[int64]models.ZondDatabaseBlock, len(blocks))

	for _, block := range blocks {
		if block.Result.Hash == "" {
			return nil, fmt.Errorf("block %s has an empty hash", block.Result.Number)
		}
		numberInt, err := parseStoredBlockNumber(block.Result.Number)
		if err != nil {
			return nil, err
		}
		if block.Result.Number != canonicalStoredBlockNumber(numberInt) {
			return nil, fmt.Errorf("block number %q is not canonical", block.Result.Number)
		}
		if previous, ok := seen[numberInt]; ok {
			if !strings.EqualFold(previous.Result.Hash, block.Result.Hash) {
				return nil, &BlockHeightConflictError{
					Number:   block.Result.Number,
					Expected: previous.Result.Hash,
					Found:    []string{block.Result.Hash},
				}
			}
			continue
		}
		seen[numberInt] = block
		candidates = append(candidates, blockWriteCandidate{block: block, numberInt: numberInt})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].numberInt < candidates[j].numberInt
	})
	for index := 1; index < len(candidates); index++ {
		if candidates[index].numberInt != candidates[index-1].numberInt+1 {
			return nil, fmt.Errorf("block batch is not contiguous: %s followed by %s",
				candidates[index-1].block.Result.Number,
				candidates[index].block.Result.Number)
		}
	}
	return candidates, nil
}

func readBlockHashState(
	ctx context.Context,
	numbers []string,
) (map[string][]storedBlockState, error) {
	if len(numbers) == 0 {
		return map[string][]storedBlockState{}, nil
	}
	numberInts := make([]int64, len(numbers))
	for index, number := range numbers {
		parsed, err := parseStoredBlockNumber(number)
		if err != nil {
			return nil, err
		}
		numberInts[index] = parsed
	}
	findOpts := options.Find().SetProjection(bson.M{
		"result.number":  1,
		"result.hash":    1,
		"ingestionState": 1,
		"_id":            0,
	})
	cursor, err := configs.BlocksCollections.Find(
		ctx,
		bson.M{"$or": bson.A{
			bson.M{"result.number": bson.M{"$in": numbers}},
			bson.M{"blockNumberInt": bson.M{"$in": numberInts}},
		}},
		findOpts,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stored []struct {
		Result struct {
			Number string `bson:"number"`
			Hash   string `bson:"hash"`
		} `bson:"result"`
		IngestionState string `bson:"ingestionState"`
	}
	if err := cursor.All(ctx, &stored); err != nil {
		return nil, err
	}

	state := make(map[string][]storedBlockState, len(stored))
	for _, block := range stored {
		numberInt, err := parseStoredBlockNumber(block.Result.Number)
		if err != nil {
			return nil, fmt.Errorf("stored block has %w", err)
		}
		number := canonicalStoredBlockNumber(numberInt)
		state[number] = append(state[number], storedBlockState{
			hash:           block.Result.Hash,
			ingestionState: block.IngestionState,
		})
	}
	return state, nil
}

func hashStateConflict(number, expected string, found []storedBlockState) error {
	foundHashes := make([]string, 0, len(found))
	conflict := len(found) != 1
	for _, stored := range found {
		foundHashes = append(foundHashes, stored.hash)
		if !strings.EqualFold(stored.hash, expected) {
			conflict = true
		}
	}
	if conflict {
		sort.Strings(foundHashes)
		return &BlockHeightConflictError{
			Number:   number,
			Expected: expected,
			Found:    foundHashes,
		}
	}
	return nil
}

func blockCompanionsComplete(number string, found []storedBlockState) (bool, error) {
	complete := true
	for _, stored := range found {
		switch stored.ingestionState {
		case BlockIngestionComplete:
		case BlockIngestionPending:
			complete = false
		case "":
			return false, fmt.Errorf("%w: block %s has no ingestionState; run the block companion reindex before restarting sync",
				ErrBlockMigrationRequired, number)
		default:
			return false, fmt.Errorf("%w: block %s has unsupported ingestionState %q; run the block companion reindex before restarting sync",
				ErrBlockMigrationRequired, number, stored.ingestionState)
		}
	}
	return complete, nil
}

func pendingOnlyBlockState(number string, found []storedBlockState) (bool, error) {
	if len(found) == 0 {
		return false, fmt.Errorf("%w at block %s", ErrCanonicalBlockNotFound, number)
	}
	for _, stored := range found {
		switch stored.ingestionState {
		case BlockIngestionPending:
		case BlockIngestionComplete:
			return false, nil
		case "":
			return false, fmt.Errorf("%w: block %s has no ingestionState",
				ErrBlockMigrationRequired, number)
		default:
			return false, fmt.Errorf("%w: block %s has unsupported ingestionState %q",
				ErrBlockMigrationRequired, number, stored.ingestionState)
		}
	}
	return true, nil
}

// BlockHeightHasOnlyPendingRows reports whether a conflicting height can be
// safely removed back to the durable cursor and fetched again.
func BlockHeightHasOnlyPendingRows(blockNumber string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	numberInt, err := parseStoredBlockNumber(blockNumber)
	if err != nil {
		return false, err
	}
	blockNumber = canonicalStoredBlockNumber(numberInt)
	state, err := readBlockHashState(ctx, []string{blockNumber})
	if err != nil {
		return false, err
	}
	return pendingOnlyBlockState(blockNumber, state[blockNumber])
}

func blockMigrationRequiredError(count int64) error {
	return fmt.Errorf("%w: %d block rows have missing or unsupported ingestionState; stop the syncer and run the documented block companion reindex before restart",
		ErrBlockMigrationRequired, count)
}

func missingBlockMigrationAttestationError() error {
	return fmt.Errorf("%w: canonical companion audit attestation is missing; run cmd/reindex-block-companions --execute before restart",
		ErrBlockMigrationRequired)
}

func hasUniqueBlockHeightIndex(specifications []*mongo.IndexSpecification) bool {
	for _, specification := range specifications {
		if specification != nil && specification.Name == BlockHeightUniqueIndex &&
			specification.Unique != nil && *specification.Unique {
			return true
		}
	}
	return false
}

func validateUniqueBlockHeightIndex(ctx context.Context) error {
	specifications, err := configs.BlocksCollections.Indexes().ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("inspect unique block-height index: %w", err)
	}
	if !hasUniqueBlockHeightIndex(specifications) {
		return fmt.Errorf("%w: unique block-height index %s is missing; run cmd/reindex-block-companions --execute while services are stopped",
			ErrBlockMigrationRequired, BlockHeightUniqueIndex)
	}
	return nil
}

// EnsureUniqueBlockHeightIndex upgrades the historical non-unique height index
// after the offline canonical audit has rejected duplicate and conflicting rows.
func EnsureUniqueBlockHeightIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	specifications, err := configs.BlocksCollections.Indexes().ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("inspect block-height indexes: %w", err)
	}
	if hasUniqueBlockHeightIndex(specifications) {
		return nil
	}
	for _, specification := range specifications {
		if specification != nil && specification.Name == BlockHeightUniqueIndex {
			if _, err := configs.BlocksCollections.Indexes().DropOne(ctx, BlockHeightUniqueIndex); err != nil {
				return fmt.Errorf("drop legacy non-unique block-height index: %w", err)
			}
			break
		}
	}
	_, err = configs.BlocksCollections.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "blockNumberInt", Value: -1}},
		Options: options.Index().SetName(BlockHeightUniqueIndex).SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("%w: create unique block-height index after canonical audit: %v",
			ErrBlockMigrationRequired, err)
	}
	return validateUniqueBlockHeightIndex(ctx)
}

func writeBlockIngestionMigrationAttestation(auditedThrough string, origin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	_, err := configs.GetCollection(configs.DB, SyncStateCollection).UpdateOne(
		ctx,
		bson.M{"_id": BlockIngestionMigrationID},
		bson.M{"$set": bson.M{
			"schema":          BlockIngestionMigrationID,
			"status":          BlockIngestionComplete,
			"audited_through": auditedThrough,
			"origin":          origin,
			"completed_at":    time.Now().UTC(),
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

// MarkBlockIngestionMigrationComplete persists the audit attestation only
// after the offline command has verified the full canonical stored range.
func MarkBlockIngestionMigrationComplete(auditedThrough string) error {
	if _, err := parseStoredBlockNumber(auditedThrough); err != nil {
		return err
	}
	return writeBlockIngestionMigrationAttestation(auditedThrough, "offline-reindex")
}

// ValidateBlockIngestionMigration refuses normal sync until every legacy row
// has passed an explicit companion reindex, carries a supported marker, and
// has a successful canonical audit attestation. An empty native database is
// attested before its first block is written.
func ValidateBlockIngestionMigration() error {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	if err := validateUniqueBlockHeightIndex(ctx); err != nil {
		return err
	}
	total, err := configs.BlocksCollections.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("count blocks for ingestion migration gate: %w", err)
	}
	count, err := configs.BlocksCollections.CountDocuments(ctx, bson.M{
		"ingestionState": bson.M{"$nin": bson.A{
			BlockIngestionPending,
			BlockIngestionComplete,
		}},
	})
	if err != nil {
		return fmt.Errorf("check block ingestion migration gate: %w", err)
	}
	if count > 0 {
		return blockMigrationRequiredError(count)
	}
	if total == 0 {
		return writeBlockIngestionMigrationAttestation("0x0", "native-empty")
	}
	var attestation struct {
		Schema  string `bson:"schema"`
		Status  string `bson:"status"`
		Through string `bson:"audited_through"`
	}
	err = configs.GetCollection(configs.DB, SyncStateCollection).FindOne(
		ctx,
		bson.M{"_id": BlockIngestionMigrationID},
	).Decode(&attestation)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return missingBlockMigrationAttestationError()
	}
	if err != nil {
		return fmt.Errorf("read block ingestion migration attestation: %w", err)
	}
	if attestation.Schema != BlockIngestionMigrationID ||
		attestation.Status != BlockIngestionComplete || attestation.Through == "" {
		return fmt.Errorf("%w: block ingestion migration attestation is malformed",
			ErrBlockMigrationRequired)
	}
	return nil
}

// BlockCompanionReindexCandidate identifies a stored block that has not yet
// crossed the durable companion completion boundary.
type BlockCompanionReindexCandidate struct {
	Number string
	Hash   string
}

func blockNeedsCompanionReindex(
	number string,
	expectedHash string,
	found []storedBlockState,
) (bool, error) {
	if len(found) == 0 {
		return false, fmt.Errorf("%w at block %s", ErrCanonicalBlockNotFound, number)
	}
	if err := hashStateConflict(number, expectedHash, found); err != nil {
		return false, err
	}
	needsReindex := false
	for _, stored := range found {
		switch stored.ingestionState {
		case BlockIngestionComplete:
		case BlockIngestionPending, "":
			needsReindex = true
		default:
			return false, fmt.Errorf("%w: block %s has unsupported ingestionState %q",
				ErrBlockMigrationRequired, number, stored.ingestionState)
		}
	}
	return needsReindex, nil
}

// ListBlockCompanionReindexCandidates returns canonical candidates in numeric
// order. A zero limit returns every candidate for a dry-run inventory.
func ListBlockCompanionReindexCandidates(limit int) ([]BlockCompanionReindexCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	findOptions := options.Find().
		SetProjection(bson.M{
			"blockNumberInt": 1,
			"result.number":  1,
			"result.hash":    1,
			"ingestionState": 1,
			"_id":            0,
		}).
		SetSort(bson.D{{Key: "blockNumberInt", Value: 1}})
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	cursor, err := configs.BlocksCollections.Find(
		ctx,
		bson.M{"ingestionState": bson.M{"$ne": BlockIngestionComplete}},
		findOptions,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type candidateRow struct {
		BlockNumberInt int64 `bson:"blockNumberInt"`
		Result         struct {
			Number string `bson:"number"`
			Hash   string `bson:"hash"`
		} `bson:"result"`
		IngestionState string `bson:"ingestionState"`
	}
	byHeight := make(map[int64]BlockCompanionReindexCandidate)
	for cursor.Next(ctx) {
		var row candidateRow
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		numberInt, err := parseStoredBlockNumber(row.Result.Number)
		if err != nil || numberInt != row.BlockNumberInt ||
			row.Result.Number != canonicalStoredBlockNumber(row.BlockNumberInt) {
			return nil, fmt.Errorf("%w: malformed block identity %q at numeric height %d",
				ErrBlockMigrationRequired, row.Result.Number, row.BlockNumberInt)
		}
		if row.Result.Hash == "" {
			return nil, fmt.Errorf("%w: block %s has an empty hash",
				ErrBlockMigrationRequired, row.Result.Number)
		}
		if row.IngestionState != "" && row.IngestionState != BlockIngestionPending {
			return nil, fmt.Errorf("%w: block %s has unsupported ingestionState %q",
				ErrBlockMigrationRequired, row.Result.Number, row.IngestionState)
		}
		if existing, ok := byHeight[numberInt]; ok {
			return nil, &BlockHeightConflictError{
				Number:   row.Result.Number,
				Expected: existing.Hash,
				Found:    []string{existing.Hash, row.Result.Hash},
			}
		}
		byHeight[numberInt] = BlockCompanionReindexCandidate{
			Number: row.Result.Number,
			Hash:   row.Result.Hash,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	heights := make([]int64, 0, len(byHeight))
	for height := range byHeight {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	candidates := make([]BlockCompanionReindexCandidate, 0, len(heights))
	for _, height := range heights {
		candidates = append(candidates, byHeight[height])
	}
	return candidates, nil
}

// CountBlockCompanionReindexRows returns the exact row count for dry-run
// inventory without loading every legacy block into memory.
func CountBlockCompanionReindexRows() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	return configs.BlocksCollections.CountDocuments(ctx,
		bson.M{"ingestionState": bson.M{"$ne": BlockIngestionComplete}})
}

type canonicalBlockAuditRow struct {
	BlockNumberInt int64 `bson:"blockNumberInt"`
	Result         struct {
		Number     string `bson:"number"`
		Hash       string `bson:"hash"`
		ParentHash string `bson:"parenthash"`
	} `bson:"result"`
	IngestionState string `bson:"ingestionState"`
}

type canonicalChainAudit struct {
	started        bool
	currentHeight  int64
	currentNumber  string
	currentHash    string
	currentParent  string
	expectedHeight int64
	previousHeight int64
	previousHash   string
	missing        []string
}

func (audit *canonicalChainAudit) add(row canonicalBlockAuditRow) error {
	if row.BlockNumberInt < 0 || row.Result.Number != canonicalStoredBlockNumber(row.BlockNumberInt) {
		return fmt.Errorf("malformed block identity %q at numeric height %d",
			row.Result.Number, row.BlockNumberInt)
	}
	if row.Result.Hash == "" {
		return fmt.Errorf("block %s has an empty hash", row.Result.Number)
	}
	if row.IngestionState != BlockIngestionComplete {
		return fmt.Errorf("block %s is not companion-complete", row.Result.Number)
	}
	if !audit.started {
		audit.startGroup(row)
		return nil
	}
	if row.BlockNumberInt < audit.currentHeight {
		return fmt.Errorf("block audit rows are not numerically ordered")
	}
	if row.BlockNumberInt == audit.currentHeight {
		return &BlockHeightConflictError{
			Number:   row.Result.Number,
			Expected: audit.currentHash,
			Found:    []string{audit.currentHash, row.Result.Hash},
		}
	}
	if err := audit.flushGroup(); err != nil {
		return err
	}
	audit.startGroup(row)
	return nil
}

func (audit *canonicalChainAudit) startGroup(row canonicalBlockAuditRow) {
	audit.started = true
	audit.currentHeight = row.BlockNumberInt
	audit.currentNumber = row.Result.Number
	audit.currentHash = row.Result.Hash
	audit.currentParent = row.Result.ParentHash
}

func (audit *canonicalChainAudit) flushGroup() error {
	for audit.expectedHeight < audit.currentHeight {
		audit.missing = append(audit.missing,
			canonicalStoredBlockNumber(audit.expectedHeight))
		audit.expectedHeight++
	}
	if audit.currentHeight > 0 && audit.previousHeight == audit.currentHeight-1 &&
		!strings.EqualFold(audit.currentParent, audit.previousHash) {
		return fmt.Errorf("block %s parent mismatch: expected %s, got %s",
			audit.currentNumber, audit.previousHash, audit.currentParent)
	}
	audit.previousHeight = audit.currentHeight
	audit.previousHash = audit.currentHash
	audit.expectedHeight = audit.currentHeight + 1
	return nil
}

func (audit *canonicalChainAudit) finish() ([]string, error) {
	if audit.started {
		if err := audit.flushGroup(); err != nil {
			return nil, err
		}
	}
	return audit.missing, nil
}

func (audit *canonicalChainAudit) finishThrough(targetHeight int64) ([]string, error) {
	missing, err := audit.finish()
	if err != nil {
		return nil, err
	}
	for audit.expectedHeight <= targetHeight {
		missing = append(missing, canonicalStoredBlockNumber(audit.expectedHeight))
		audit.expectedHeight++
	}
	return missing, nil
}

// AuditCompletedCanonicalBlockChain verifies numeric continuity, unique
// identity, parent linkage, and durable completion across every stored height.
// Missing heights are returned for ordered RPC repair.
func AuditCompletedCanonicalBlockChain() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cursor, err := configs.BlocksCollections.Find(
		ctx,
		bson.M{},
		options.Find().
			SetProjection(bson.M{
				"blockNumberInt":    1,
				"result.number":     1,
				"result.hash":       1,
				"result.parenthash": 1,
				"ingestionState":    1,
				"_id":               0,
			}).
			SetSort(bson.D{{Key: "blockNumberInt", Value: 1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	audit := &canonicalChainAudit{previousHeight: -1}
	for cursor.Next(ctx) {
		var row canonicalBlockAuditRow
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		if err := audit.add(row); err != nil {
			return nil, err
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	syncBlock, err := GetLastKnownBlockNumberStrict()
	if errors.Is(err, ErrSyncStateNotFound) {
		if audit.started {
			return nil, fmt.Errorf("%w for a nonempty block collection", ErrSyncStateNotFound)
		}
		return audit.finish()
	}
	if err != nil {
		return nil, err
	}
	syncHeight, err := parseStoredBlockNumber(syncBlock)
	if err != nil {
		return nil, fmt.Errorf("parse durable sync cursor for canonical audit: %w", err)
	}
	return audit.finishThrough(syncHeight)
}

// PrepareBlockCompanionReindex atomically converts exact markerless rows to
// pending. The pending state itself is the resumable checkpoint.
func PrepareBlockCompanionReindex(blockNumber, blockHash string) (bool, error) {
	numberInt, err := parseStoredBlockNumber(blockNumber)
	if err != nil {
		return false, err
	}
	blockNumber = canonicalStoredBlockNumber(numberInt)
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	state, err := readBlockHashState(ctx, []string{blockNumber})
	if err != nil {
		return false, err
	}
	needsReindex, err := blockNeedsCompanionReindex(blockNumber, blockHash, state[blockNumber])
	if err != nil || !needsReindex {
		return needsReindex, err
	}
	identityFilter, err := blockIdentityFilter(blockNumber, blockHash)
	if err != nil {
		return false, err
	}
	_, err = configs.BlocksCollections.UpdateMany(ctx, bson.M{"$and": bson.A{
		identityFilter,
		bson.M{"$or": bson.A{
			bson.M{"ingestionState": bson.M{"$exists": false}},
			bson.M{"ingestionState": ""},
		}},
	}}, bson.M{"$set": bson.M{"ingestionState": BlockIngestionPending}})
	if err != nil {
		return false, err
	}
	state, err = readBlockHashState(ctx, []string{blockNumber})
	if err != nil {
		return false, err
	}
	return blockNeedsCompanionReindex(blockNumber, blockHash, state[blockNumber])
}

func classifyConfirmedBlockPrefix(
	candidates []blockWriteCandidate,
	existedBefore map[string]bool,
	observed map[string][]storedBlockState,
) (BlockBatchWriteResult, error) {
	result := BlockBatchWriteResult{Attempted: len(candidates)}
	for _, candidate := range candidates {
		number := candidate.block.Result.Number
		found := observed[number]
		if len(found) == 0 {
			return result, fmt.Errorf("%w at block %s", ErrBlockWriteUnresolved, number)
		}
		if err := hashStateConflict(number, candidate.block.Result.Hash, found); err != nil {
			return result, err
		}
		companionsComplete, err := blockCompanionsComplete(number, found)
		if err != nil {
			return result, err
		}
		result.Confirmed = append(result.Confirmed, ConfirmedBlockWrite{
			Block:              candidate.block,
			AlreadyExisted:     existedBefore[number],
			CompanionsComplete: companionsComplete,
		})
	}
	return result, nil
}

// GetCanonicalBlockHash returns the sole durably completed hash identity at a
// height. Duplicate rows and any pending row block child ingestion.
func GetCanonicalBlockHash(blockNumber string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()

	numberInt, err := parseStoredBlockNumber(blockNumber)
	if err != nil {
		return "", err
	}
	blockNumber = canonicalStoredBlockNumber(numberInt)
	state, err := readBlockHashState(ctx, []string{blockNumber})
	if err != nil {
		return "", err
	}
	found := state[blockNumber]
	if len(found) == 0 {
		return "", fmt.Errorf("%w at block %s", ErrCanonicalBlockNotFound, blockNumber)
	}
	expected := found[0].hash
	if err := hashStateConflict(blockNumber, expected, found); err != nil {
		return "", err
	}
	complete, err := blockCompanionsComplete(blockNumber, found)
	if err != nil {
		return "", err
	}
	if !complete {
		return "", fmt.Errorf("%w: canonical block %s has incomplete companions",
			ErrBlockWriteUnresolved, blockNumber)
	}
	return expected, nil
}

// InsertManyBlockDocuments writes blocks in ascending order and confirms the
// exact stored height and hash identities afterward. Ordered writes ensure a
// server-side partial outcome can only create a prefix of the new candidates.
func InsertManyBlockDocuments(blocks []models.ZondDatabaseBlock) (BlockBatchWriteResult, error) {
	candidates, err := sortedBlockWriteCandidates(blocks)
	if err != nil {
		return BlockBatchWriteResult{}, err
	}
	if len(candidates) == 0 {
		return BlockBatchWriteResult{}, nil
	}

	numbers := make([]string, len(candidates))
	for i, candidate := range candidates {
		numbers[i] = candidate.block.Result.Number
	}

	beforeCtx, beforeCancel := context.WithTimeout(context.Background(), DBTimeout)
	before, err := readBlockHashState(beforeCtx, numbers)
	beforeCancel()
	if err != nil {
		return BlockBatchWriteResult{Attempted: len(candidates)},
			fmt.Errorf("read block identities before batch write: %w", err)
	}

	existedBefore := make(map[string]bool, len(candidates))
	documents := make([]interface{}, 0, len(candidates))
	for _, candidate := range candidates {
		number := candidate.block.Result.Number
		found := before[number]
		if len(found) > 0 {
			if err := hashStateConflict(number, candidate.block.Result.Hash, found); err != nil {
				return BlockBatchWriteResult{Attempted: len(candidates)}, err
			}
			existedBefore[number] = true
			continue
		}
		documents = append(documents, models.ZondDatabaseBlockWithInt{
			Jsonrpc:             candidate.block.Jsonrpc,
			ID:                  candidate.block.ID,
			Result:              candidate.block.Result,
			BlockNumberInt:      candidate.numberInt,
			IngestionState:      BlockIngestionPending,
			TokenIngestionState: "pending",
		})
	}

	var writeErr error
	if len(documents) > 0 {
		writeCtx, writeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, writeErr = configs.BlocksCollections.InsertMany(
			writeCtx,
			documents,
			options.InsertMany().SetOrdered(true),
		)
		writeCancel()
	}

	afterCtx, afterCancel := context.WithTimeout(context.Background(), DBTimeout)
	after, readErr := readBlockHashState(afterCtx, numbers)
	afterCancel()
	if readErr != nil {
		// Before-state rows remain confirmed. New candidates remain unresolved
		// until a post-write read proves their exact identities.
		after = before
	}
	result, classifyErr := classifyConfirmedBlockPrefix(candidates, existedBefore, after)
	if classifyErr != nil {
		return result, errors.Join(classifyErr, writeErr, readErr)
	}
	if readErr != nil {
		return result, errors.Join(ErrBlockWriteUnresolved, writeErr, readErr)
	}
	if len(result.Confirmed) != result.Attempted {
		return result, errors.Join(ErrBlockWriteUnresolved, writeErr, readErr)
	}
	if writeErr != nil {
		configs.Logger.Warn("Block batch write returned an error; post-write identities are fully confirmed",
			zap.Error(writeErr))
	}
	return result, nil
}

func blockIdentityFilter(blockNumber, blockHash string) (bson.M, error) {
	numberInt, err := parseStoredBlockNumber(blockNumber)
	if err != nil {
		return nil, err
	}
	return bson.M{
		"$and": bson.A{
			bson.M{"$or": bson.A{
				bson.M{"result.number": canonicalStoredBlockNumber(numberInt)},
				bson.M{"blockNumberInt": numberInt},
			}},
			bson.M{"result.hash": primitive.Regex{
				Pattern: "^" + regexp.QuoteMeta(blockHash) + "$",
				Options: "i",
			}},
		},
	}, nil
}

// MarkBlockCompanionsComplete commits the durable boundary that permits sync
// state to advance. The same atomic update initializes the durable token scan
// for new and migrated blocks while preserving a completed token marker.
func MarkBlockCompanionsComplete(blockNumber, blockHash string) error {
	filter, err := blockIdentityFilter(blockNumber, blockHash)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), DBTimeout)
	defer cancel()
	result, err := configs.BlocksCollections.UpdateOne(ctx, filter, mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"ingestionState":        BlockIngestionComplete,
			"companionsCompletedAt": time.Now().UTC(),
			"tokenIngestionState": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{
					bson.M{"$size": bson.M{"$ifNull": bson.A{"$result.transactions", bson.A{}}}},
					0,
				}},
				"complete",
				bson.M{"$ifNull": bson.A{"$tokenIngestionState", "pending"}},
			}},
		}}},
		bson.D{{Key: "$set", Value: bson.M{
			"tokenCompletedAt": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$tokenIngestionState", "complete"}},
				bson.M{"$ifNull": bson.A{"$tokenCompletedAt", "$$NOW"}},
				"$$REMOVE",
			}},
		}}},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("%w: exact block %s %s disappeared before completion",
			ErrBlockWriteUnresolved, blockNumber, blockHash)
	}
	state, err := readBlockHashState(ctx, []string{blockNumber})
	if err != nil {
		return fmt.Errorf("confirm companion marker for block %s: %w", blockNumber, err)
	}
	found := state[blockNumber]
	if len(found) == 0 {
		return fmt.Errorf("%w: block %s disappeared after companion completion",
			ErrBlockWriteUnresolved, blockNumber)
	}
	if err := hashStateConflict(blockNumber, blockHash, found); err != nil {
		return err
	}
	complete, err := blockCompanionsComplete(blockNumber, found)
	if err != nil {
		return err
	}
	if !complete {
		return fmt.Errorf("%w: block %s retains a pending companion marker",
			ErrBlockWriteUnresolved, blockNumber)
	}
	return nil
}

// ResetBlockCompanionRows removes append-only rows from an interrupted block
// attempt. Contract and address stores use keyed upserts and are safe to replay.
func ResetBlockCompanionRows(blockNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session, err := configs.DB.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"blockNumber": blockNumber}
		collections := []*mongo.Collection{
			configs.TransferCollections,
			configs.TransactionByAddressCollections,
			configs.InternalTransactionByAddressCollections,
			configs.GetCollection(configs.DB, "pending_token_contracts"),
		}
		for _, collection := range collections {
			if _, err := collection.DeleteMany(sessCtx, filter); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// Rollback removes all blocks after the given block number and updates the sync state.
// Filter on blockNumberInt (numeric), NOT result.number (hex string), lex comparison
// on hex strings produces wrong results (e.g. "0xa" > "0x100" lexicographically),
// which would leave stale blocks above the rollback point during chain reorgs and
// silently corrupt the chain view.
func Rollback(blockNumber string) error {
	// Validate the input BEFORE building the filter. HexToInt64 / HexToInt
	// both silently return 0 on parse failure (empty string, missing 0x,
	// non-hex chars). With a $gt: 0 filter, a malformed blockNumber would
	// delete every block except genesis, strictly worse than the original
	// lex-sort bug we were fixing. Parse explicitly so we get an error path,
	// and reject negative values defensively (e.g. "0x-10" → -16 via
	// strconv) which would also widen the filter dangerously.
	s := strings.TrimPrefix(blockNumber, "0x")
	if s == "" {
		return fmt.Errorf("rollback: empty block number")
	}
	rollbackTo, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return fmt.Errorf("rollback: invalid hex block number %q: %w", blockNumber, err)
	}
	if rollbackTo < 0 {
		return fmt.Errorf("rollback: block number cannot be negative: %d", rollbackTo)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	filter := bson.M{
		"blockNumberInt": bson.M{
			"$gt": rollbackTo,
		},
	}

	// Delete blocks in a transaction
	session, err := configs.DB.StartSession()
	if err != nil {
		configs.Logger.Error("Failed to start session for rollback",
			zap.Error(err))
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Resolve the exact rollback set inside the same transaction snapshot
		// used for every deletion. A concurrent gap-fill writer cannot create a
		// block that the broad delete sees while its companion identity is absent
		// from this list.
		cursor, err := configs.BlocksCollections.Find(sessCtx, filter)
		if err != nil {
			return nil, fmt.Errorf("rollback: failed to find doomed blocks: %w", err)
		}
		defer cursor.Close(sessCtx)

		var blocks []models.ZondDatabaseBlock
		if err := cursor.All(sessCtx, &blocks); err != nil {
			return nil, fmt.Errorf("rollback: failed to decode doomed blocks: %w", err)
		}

		// Companion collections store blockNumber as raw hex, so use the exact
		// set captured from this snapshot. Numeric range comparisons against hex
		// strings would be unsafe at width boundaries.
		badBlockNumbers := make([]string, 0, len(blocks))
		badBlockHashes := make([]string, 0, len(blocks))
		badTransactionHashes := make([]string, 0)
		nativeBalanceAddresses := make([]string, 0)
		for _, block := range blocks {
			if block.Result.Number != "" {
				badBlockNumbers = append(badBlockNumbers, block.Result.Number)
			}
			if block.Result.Hash != "" {
				badBlockHashes = append(badBlockHashes, block.Result.Hash)
			}
			configs.Logger.Info("Rolling back block",
				zap.String("number", block.Result.Number),
				zap.String("hash", block.Result.Hash))
			nativeBalanceAddresses = appendRollbackNativeAddress(
				nativeBalanceAddresses,
				block.Result.Miner,
			)
			for _, withdrawal := range block.Result.Withdrawals {
				nativeBalanceAddresses = appendRollbackNativeAddress(
					nativeBalanceAddresses,
					withdrawal.Address,
				)
			}
			for _, tx := range block.Result.Transactions {
				if tx.Hash != "" {
					badTransactionHashes = append(badTransactionHashes, tx.Hash)
				}
				nativeBalanceAddresses = appendRollbackNativeAddress(
					nativeBalanceAddresses,
					tx.From,
				)
				nativeBalanceAddresses = appendRollbackNativeAddress(
					nativeBalanceAddresses,
					tx.To,
				)
			}
		}

		// A mined pending row is a tombstone that prevents a stale mempool poll
		// from resurrecting the transaction. Once its canonical block is
		// orphaned, remove only that mined tombstone so the transaction can
		// become pending again if the node advertises it on the new fork.
		if len(badTransactionHashes) > 0 {
			if _, err := configs.PendingTransactionsCollections.DeleteMany(
				sessCtx,
				bson.M{
					"_id":    bson.M{"$in": badTransactionHashes},
					"status": "mined",
				},
			); err != nil {
				return nil, fmt.Errorf("rollback: failed to remove mined pending tombstones: %w", err)
			}
		}

		// Delete blocks
		_, err = configs.BlocksCollections.DeleteMany(sessCtx, filter)
		if err != nil {
			return nil, err
		}

		// Delete the append-only companion rows for the removed blocks so a
		// rolled-back block leaves no orphan event records. These collections
		// key blockNumber as the raw hex string, so the exact-match $in filter
		// is correct (a numeric range would lex-sort hex strings wrong).
		//
		// Snapshot balances cannot be deleted by block number. Capture every
		// affected native and token identity before companion deletion, mark the
		// snapshots stale, and enqueue canonical RPC reconciliation in this same
		// transaction. Public readers exclude those snapshots until refresh.
		if len(badBlockNumbers) > 0 {
			companionFilter := bson.M{"blockNumber": bson.M{"$in": badBlockNumbers}}
			createdContractAddresses := make([]string, 0)

			// Resolve contract addresses before deleting their creation evidence.
			// Direct deployments store contractAddress on the outer transfer; factory
			// deployments store the CREATE/CREATE2 target in the trace index.
			directAddresses, err := configs.TransferCollections.Distinct(
				sessCtx,
				"contractAddress",
				bson.M{
					"blockNumber":     bson.M{"$in": badBlockNumbers},
					"contractAddress": bson.M{"$exists": true, "$ne": ""},
				},
			)
			if err != nil {
				return nil, fmt.Errorf("rollback: failed to resolve direct contract creations: %w", err)
			}
			createdContractAddresses = appendStringValues(createdContractAddresses, directAddresses)

			internalAddresses, err := configs.InternalTransactionByAddressCollections.Distinct(
				sessCtx,
				"to",
				bson.M{
					"blockNumber": bson.M{"$in": badBlockNumbers},
					"type":        bson.M{"$in": []string{"CREATE", "CREATE2"}},
					"to":          bson.M{"$ne": ""},
				},
			)
			if err != nil {
				return nil, fmt.Errorf("rollback: failed to resolve traced contract creations: %w", err)
			}
			createdContractAddresses = appendStringValues(createdContractAddresses, internalAddresses)

			// Contract documents carry their own creation identity and can survive
			// an interrupted companion write. Include them directly so rollback
			// invalidates a deployment even if its transfer or trace row is absent.
			contractCreationFilter := contractRollbackFilter(
				badBlockNumbers,
				badBlockHashes,
				badTransactionHashes,
				nil,
			)
			contractDocumentAddresses, err := configs.GetContractsCollection().Distinct(
				sessCtx,
				"address",
				contractCreationFilter,
			)
			if err != nil {
				return nil, fmt.Errorf("rollback: failed to resolve contract creation records: %w", err)
			}
			createdContractAddresses = appendStringValues(
				createdContractAddresses,
				contractDocumentAddresses,
			)
			for _, address := range createdContractAddresses {
				nativeBalanceAddresses = appendRollbackNativeAddress(
					nativeBalanceAddresses,
					address,
				)
			}

			// Internal value moves may touch addresses that are absent from the
			// outer transaction. Mark both sides if a snapshot exists.
			for _, field := range []string{"from", "to"} {
				values, err := configs.InternalTransactionByAddressCollections.Distinct(
					sessCtx,
					field,
					companionFilter,
				)
				if err != nil {
					return nil, fmt.Errorf("rollback: failed to resolve internal %s addresses: %w", field, err)
				}
				for _, value := range values {
					address, ok := value.(string)
					if !ok {
						continue
					}
					nativeBalanceAddresses = appendRollbackNativeAddress(
						nativeBalanceAddresses,
						address,
					)
				}
			}

			tokenTransfersCollection := configs.GetTokenTransfersCollection()
			tokenCursor, err := tokenTransfersCollection.Find(sessCtx, companionFilter)
			if err != nil {
				return nil, fmt.Errorf("rollback: failed to find orphaned token transfers: %w", err)
			}
			var orphanedTokenTransfers []models.TokenTransfer
			if err := tokenCursor.All(sessCtx, &orphanedTokenTransfers); err != nil {
				tokenCursor.Close(sessCtx)
				return nil, fmt.Errorf("rollback: failed to decode orphaned token transfers: %w", err)
			}
			if err := tokenCursor.Close(sessCtx); err != nil {
				return nil, fmt.Errorf("rollback: failed to close orphaned token transfer cursor: %w", err)
			}

			tokenBalanceItems := balanceReconciliationsForTokenTransfers(
				orphanedTokenTransfers,
				blockNumber,
			)
			balanceItems := append([]balanceReconciliation(nil), tokenBalanceItems...)
			for _, address := range nativeBalanceAddresses {
				if item, ok := newNativeBalanceReconciliation(address, blockNumber); ok {
					balanceItems = append(balanceItems, item)
				}
			}
			if err := markNativeBalancesStale(sessCtx, nativeBalanceAddresses); err != nil {
				return nil, fmt.Errorf("rollback: failed to mark native balances stale: %w", err)
			}
			if err := markTokenBalancesStale(sessCtx, tokenBalanceItems); err != nil {
				return nil, fmt.Errorf("rollback: failed to mark token balances stale: %w", err)
			}
			if err := enqueueBalanceReconciliations(sessCtx, balanceItems); err != nil {
				return nil, fmt.Errorf("rollback: failed to enqueue balance reconciliation: %w", err)
			}

			// A contract document contains backend-owned source verification, ABI,
			// and AI state. Remove the whole row when its creation was orphaned so
			// the canonical replay starts from an empty trust record. Match by every
			// creation identity available so legacy rows missing one field are still
			// invalidated.
			contractFilter := contractRollbackFilter(
				badBlockNumbers,
				badBlockHashes,
				badTransactionHashes,
				createdContractAddresses,
			)
			if len(contractFilter) > 0 {
				jobFilter := verificationJobRollbackFilter(
					badBlockNumbers,
					badBlockHashes,
					badTransactionHashes,
					createdContractAddresses,
				)
				if _, err := configs.GetCollection(
					configs.DB,
					configs.CONTRACT_VERIFICATIONS_COLLECTION,
				).UpdateMany(
					sessCtx,
					jobFilter,
					bson.M{
						"$set": bson.M{
							"status":    "failed",
							"error":     "verification cancelled: canonical contract creation was orphaned by rollback",
							"updatedAt": time.Now().UTC().Format(time.RFC3339),
						},
						"$unset": bson.M{"result": ""},
					},
				); err != nil {
					return nil, fmt.Errorf("rollback: failed to cancel orphaned verification jobs: %w", err)
				}
				if _, err := configs.GetContractsCollection().DeleteMany(sessCtx, contractFilter); err != nil {
					return nil, fmt.Errorf("rollback: failed to invalidate orphaned contract trust records: %w", err)
				}
			}

			// The addresses collection latches isContract=true. Clear that snapshot
			// for orphaned creations; canonical replay promotes it again if the new
			// fork recreates the address.
			if len(createdContractAddresses) > 0 {
				if _, err := configs.AddressesCollections.UpdateMany(
					sessCtx,
					orphanedContractAddressFilter(createdContractAddresses),
					bson.M{"$set": bson.M{"isContract": false}},
				); err != nil {
					return nil, fmt.Errorf("rollback: failed to clear orphaned contract address flags: %w", err)
				}
			}

			if _, err := configs.TransferCollections.DeleteMany(sessCtx, companionFilter); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete transfer rows: %w", err)
			}
			if _, err := configs.TransactionByAddressCollections.DeleteMany(sessCtx, companionFilter); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete transactionByAddress rows: %w", err)
			}
			if _, err := configs.InternalTransactionByAddressCollections.DeleteMany(sessCtx, companionFilter); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete internalTransactionByAddress rows: %w", err)
			}

			if _, err := tokenTransfersCollection.DeleteMany(sessCtx, companionFilter); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete tokenTransfers rows: %w", err)
			}
			if _, err := configs.GetTokenEventDeadLettersCollection().DeleteMany(
				sessCtx,
				companionFilter,
			); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete token event dead-letters: %w", err)
			}

			pendingContracts := configs.GetCollection(configs.DB, "pending_token_contracts")
			if _, err := pendingContracts.DeleteMany(sessCtx, companionFilter); err != nil {
				return nil, fmt.Errorf("rollback: failed to delete pending contract work: %w", err)
			}
		}

		// Rollback must move the high-water mark backward. Use the transaction
		// session and an unconditional exact-target update; the ordinary sync
		// setter is monotonic-increase-only by design.
		if err := setRollbackSyncState(sessCtx, blockNumber, rollbackTo); err != nil {
			return nil, fmt.Errorf("rollback: failed to lower sync state: %w", err)
		}

		return nil, nil
	})

	if err != nil {
		configs.Logger.Error("Failed to execute rollback transaction",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully rolled back to block",
		zap.String("block_number", blockNumber))
	return nil
}

func orphanedContractAddressFilter(addresses []string) bson.M {
	return bson.M{"id": bson.M{"$in": addresses}}
}

func verificationJobRollbackFilter(
	blockNumbers,
	blockHashes,
	transactionHashes,
	addresses []string,
) bson.M {
	branches := make(bson.A, 0, 4)
	if len(blockNumbers) > 0 {
		branches = append(branches, bson.M{
			"target.creationBlockNumber": bson.M{"$in": blockNumbers},
		})
	}
	if len(blockHashes) > 0 {
		branches = append(branches, bson.M{
			"target.creationBlockHash": bson.M{"$in": blockHashes},
		})
	}
	if len(transactionHashes) > 0 {
		branches = append(branches, bson.M{
			"target.creationTransaction": bson.M{"$in": transactionHashes},
		})
	}
	if len(addresses) > 0 {
		branches = append(branches, bson.M{
			"target.address": bson.M{"$in": addresses},
		})
	}
	if len(branches) == 0 {
		return nil
	}
	return bson.M{
		"status": bson.M{"$in": []string{"pending", "compiling", "success"}},
		"$or":    branches,
	}
}

func setRollbackSyncState(ctx context.Context, blockNumber string, blockNumberInt int64) error {
	filter, update := rollbackSyncStateMutation(blockNumber, blockNumberInt)
	_, err := configs.GetCollection(configs.DB, SyncStateCollection).UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)
	return err
}

func rollbackSyncStateMutation(blockNumber string, blockNumberInt int64) (bson.M, bson.M) {
	return bson.M{"_id": LastSyncedBlockID}, bson.M{"$set": bson.M{
		"block_number":     blockNumber,
		"block_number_int": blockNumberInt,
	}}
}

func appendStringValues(dst []string, values []interface{}) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		if _, exists := seen[text]; exists {
			continue
		}
		seen[text] = struct{}{}
		dst = append(dst, text)
	}
	return dst
}

func appendRollbackNativeAddress(dst []string, address string) []string {
	if !validation.IsValidAddress(address) || validation.IsZeroAddress(address) {
		return dst
	}
	return append(dst, validation.ConvertToQAddress(address))
}

func contractRollbackFilter(blockNumbers, blockHashes, transactionHashes, addresses []string) bson.M {
	branches := make(bson.A, 0, 4)
	if len(blockNumbers) > 0 {
		branches = append(branches, bson.M{
			"creationBlockNumber": bson.M{"$in": blockNumbers},
		})
	}
	if len(blockHashes) > 0 {
		branches = append(branches, bson.M{
			"creationBlockHash": bson.M{"$in": blockHashes},
		})
	}
	if len(transactionHashes) > 0 {
		branches = append(branches, bson.M{
			"creationTransaction": bson.M{"$in": transactionHashes},
		})
	}
	if len(addresses) > 0 {
		branches = append(branches, bson.M{
			"address": bson.M{"$in": addresses},
		})
	}
	if len(branches) == 0 {
		return nil
	}
	return bson.M{"$or": branches}
}

// UpdateBlockSizeCollection updates the averageBlockSize collection with size data
// This should be called periodically to maintain up-to-date block size data
func UpdateBlockSizeCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Clean up existing data first
	_, err := configs.AverageBlockSizeCollections.DeleteMany(ctx, bson.M{})
	if err != nil {
		configs.Logger.Error("Failed to clean up block size collection",
			zap.Error(err))
		return err
	}

	// Set up aggregation pipeline to compute block sizes
	// We'll take all blocks, sort by timestamp, and include basic info and size
	pipeline := []bson.M{
		{
			"$sort": bson.M{"result.timestamp": 1},
		},
		{
			"$project": bson.M{
				"blockNumber":      "$result.number",
				"timestamp":        "$result.timestamp",
				"size":             "$result.size",
				"transactionCount": bson.M{"$size": "$result.transactions"},
			},
		},
	}

	// Execute the pipeline
	cursor, err := configs.BlocksCollections.Aggregate(ctx, pipeline)
	if err != nil {
		configs.Logger.Error("Failed to aggregate block sizes",
			zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	// Process the results
	var blockSizes []interface{}
	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			configs.Logger.Warn("Failed to decode block size",
				zap.Error(err))
			continue
		}

		// Add to our block sizes
		blockSizes = append(blockSizes, result)
	}

	// Insert the processed block sizes
	if len(blockSizes) > 0 {
		_, err = configs.AverageBlockSizeCollections.InsertMany(ctx, blockSizes)
		if err != nil {
			configs.Logger.Error("Failed to insert block sizes",
				zap.Error(err))
			return err
		}

		configs.Logger.Info("Updated block size collection",
			zap.Int("count", len(blockSizes)))
	} else {
		configs.Logger.Warn("No block sizes to update")
	}

	return nil
}
