package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/utils"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	// GenesisBlockHex is the genesis block number in hex
	GenesisBlockHex = "0x0"

	// Internal constants (not exported)
	dbTimeout          = DBTimeout
	lastSyncedBlockID  = LastSyncedBlockID
	initialSyncStartID = InitialSyncStartID
	genesisBlockHex    = GenesisBlockHex
)

// GetLatestBlockFromDB returns the latest block from the database
// Returns nil if no blocks exist or if there's an error
func GetLatestBlockFromDB() *models.ZondDatabaseBlock {
	if !IsCollectionsExist() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Query for the latest block by sorting on block number in descending order
	findOptions := options.FindOne().
		SetProjection(bson.M{"result.number": 1, "result.timestamp": 1}).
		SetSort(bson.M{"result.number": -1})

	var block models.ZondDatabaseBlock
	err := configs.BlocksCollections.FindOne(ctx, bson.D{}, findOptions).Decode(&block)
	if err != nil {
		configs.Logger.Info("Failed to find latest block", zap.Error(err))
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
	return genesisBlockHex
}

// GetLatestBlockHashHeaderFromDB returns the hash of a block with the given number
// Returns empty string if the block doesn't exist or if there's an error
func GetLatestBlockHashHeaderFromDB(blockNumber string) string {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	syncColl := configs.GetCollection(configs.DB, SyncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{
		"_id": lastSyncedBlockID,
	}).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			configs.Logger.Info("No sync state found, this appears to be the first run")
		} else {
			configs.Logger.Warn("Failed to get last known block number", zap.Error(err))
		}
		return genesisBlockHex
	}

	if result.BlockNumber == "" {
		configs.Logger.Warn("Found sync state but block number is empty")
		return genesisBlockHex
	}

	configs.Logger.Info("Found last known block in sync state",
		zap.String("block", result.BlockNumber))
	return result.BlockNumber
}

// GetLastSyncedBlock retrieves the last synced block as a ZondDatabaseBlock object
// This is a wrapper around GetLastKnownBlockNumber that returns a block object
func GetLastSyncedBlock() (*models.ZondDatabaseBlock, error) {
	blockNumber := GetLastKnownBlockNumber()

	// Create a block object with the retrieved block number
	return &models.ZondDatabaseBlock{
		Result: models.Result{
			Number: blockNumber,
		},
	}, nil
}

// StoreLastKnownBlockNumber updates the sync state with the given block number
// Only updates if the new block number is higher than the existing one
func StoreLastKnownBlockNumber(blockNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, SyncStateCollection)

	// First check if the document exists
	var existingDoc struct {
		BlockNumber string `bson:"block_number"`
	}

	err := syncColl.FindOne(ctx, bson.M{"_id": lastSyncedBlockID}).Decode(&existingDoc)

	if err == mongo.ErrNoDocuments {
		// Document doesn't exist, create it
		_, err = syncColl.InsertOne(ctx, bson.M{
			"_id":          lastSyncedBlockID,
			"block_number": blockNumber,
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

	// Document exists or was just created by another goroutine
	// Only update if the new block number is higher
	result, err := syncColl.UpdateOne(
		ctx,
		bson.M{
			"_id":          lastSyncedBlockID,
			"block_number": bson.M{"$lt": blockNumber},
		},
		bson.M{"$set": bson.M{"block_number": blockNumber}},
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	syncColl := configs.GetCollection(configs.DB, InitialSyncStateCollection)
	err := syncColl.FindOne(ctx, bson.M{
		"_id": initialSyncStartID,
	}).Decode(&result)

	if err == nil && result.BlockNumber != "" {
		configs.Logger.Info("Found initial sync start block",
			zap.String("block", result.BlockNumber))
		return result.BlockNumber
	}

	// If no record exists, find the oldest block in the DB
	var block models.ZondDatabaseBlock
	findOptions := options.FindOne().SetProjection(bson.M{"result.number": 1}).SetSort(bson.M{"result.number": 1})
	err = configs.BlocksCollections.FindOne(ctx, bson.M{}, findOptions).Decode(&block)

	if err == nil && block.Result.Number != "" {
		// Store this for future reference
		_, _ = syncColl.UpdateOne(
			ctx,
			bson.M{"_id": initialSyncStartID},
			bson.M{"$set": bson.M{"block_number": block.Result.Number}},
			options.Update().SetUpsert(true),
		)

		configs.Logger.Info("Using oldest block in DB as initial sync start",
			zap.String("block", block.Result.Number))
		return block.Result.Number
	}

	// If all else fails, start from genesis
	configs.Logger.Info("No initial sync start point found, starting from genesis")
	return genesisBlockHex
}

// StoreInitialSyncStartBlock stores the block number that was used as the starting point
// for the initial sync. This is used for token transfer processing after initial sync.
func StoreInitialSyncStartBlock(blockNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	syncColl := configs.GetCollection(configs.DB, InitialSyncStateCollection)

	// Update or insert the initial sync start block
	_, err := syncColl.UpdateOne(
		ctx,
		bson.M{"_id": initialSyncStartID},
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

// BlockExists checks if a block with the given number already exists in the database
func BlockExists(blockNumber string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a projection to only return the _id field for efficiency
	findOptions := options.FindOne().SetProjection(bson.M{"_id": 1})

	// Try to find the block
	err := configs.BlocksCollections.FindOne(
		ctx,
		bson.M{"result.number": blockNumber},
		findOptions,
	).Err()

	// If no error, the block exists
	if err == nil {
		return true
	}

	// If the error is "no documents found", the block doesn't exist
	if err == mongo.ErrNoDocuments {
		return false
	}

	// For any other error, log it and assume the block doesn't exist
	configs.Logger.Warn("Failed to check if block exists",
		zap.String("block", blockNumber),
		zap.Error(err))
	return false
}

// InsertBlockDocument inserts a single block document into the database
// It returns an error if the insertion fails for reasons other than a duplicate key.
func InsertBlockDocument(block models.ZondDatabaseBlock) error {
	// Check if block already exists before attempting insert
	if BlockExists(block.Result.Number) {
		configs.Logger.Info("Block already exists, skipping insert", zap.String("blockNumber", block.Result.Number))
		return nil // Not an error if it already exists
	}

	collection := configs.BlocksCollections
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := collection.InsertOne(ctx, block)
	if err != nil {
		// Check for duplicate key error specifically
		if mongo.IsDuplicateKeyError(err) {
			configs.Logger.Info("Attempted to insert duplicate block (race condition likely), ignoring error", zap.String("blockNumber", block.Result.Number))
			return nil // Treat duplicate key as non-fatal for this function's purpose
		} else {
			configs.Logger.Warn("Failed to insert block document", zap.String("blockNumber", block.Result.Number), zap.Error(err))
			return err // Return other errors
		}
	} else {
		configs.Logger.Info("Successfully inserted single block", zap.String("blockNumber", block.Result.Number))
	}
	return nil // Return nil on success
}

// InsertManyBlockDocuments inserts multiple block documents into the database
// Filters out blocks that already exist before inserting
func InsertManyBlockDocuments(blocks []interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Prevent duplicate insertions if the same batch is somehow processed twice
	// Use a map to track block numbers already processed in this specific batch
	processedBlockNumbers := make(map[string]bool)
	var uniqueBlocks []interface{}

	for _, blockData := range blocks {
		block, ok := blockData.(models.ZondDatabaseBlock)
		if !ok {
			// Handle type assertion failure if necessary
			configs.Logger.Error("Failed type assertion for block data")
			continue
		}

		blockNumber := block.Result.Number
		if processedBlockNumbers[blockNumber] {
			configs.Logger.Debug("Skipping duplicate block within batch", zap.String("blockNumber", blockNumber))
			continue
		}

		// Additionally check if the block already exists in the database
		if BlockExists(blockNumber) {
			configs.Logger.Debug("Block already exists in database, skipping insertion", zap.String("blockNumber", blockNumber))
			continue
		}

		uniqueBlocks = append(uniqueBlocks, blockData)
		processedBlockNumbers[blockNumber] = true
	}

	// Only insert if we have unique blocks
	if len(uniqueBlocks) > 0 {
		configs.Logger.Info("Inserting unique blocks",
			zap.Int("originalCount", len(blocks)),
			zap.Int("uniqueCount", len(uniqueBlocks)))

		_, err := configs.BlocksCollections.InsertMany(ctx, uniqueBlocks)
		if err != nil {
			configs.Logger.Warn("Failed to insert many block documents", zap.Error(err))
			return err // Return the error
		}
	} else {
		configs.Logger.Info("No unique blocks to insert",
			zap.Int("originalCount", len(blocks)))
	}
	return nil // Return nil on success
}

// Rollback removes all blocks after the given block number and updates the sync state
func Rollback(blockNumber string) error {
	ctx := context.Background()

	// Find all blocks after the given block number
	filter := bson.M{
		"result.number": bson.M{
			"$gt": blockNumber,
		},
	}

	// Get blocks to be removed for logging
	cursor, err := configs.BlocksCollections.Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to find blocks for rollback",
			zap.String("from_block", blockNumber),
			zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	var blocks []models.ZondDatabaseBlock
	if err = cursor.All(ctx, &blocks); err != nil {
		configs.Logger.Error("Failed to decode blocks for rollback",
			zap.Error(err))
		return err
	}

	// Log blocks being removed
	for _, block := range blocks {
		configs.Logger.Info("Rolling back block",
			zap.String("number", block.Result.Number),
			zap.String("hash", block.Result.Hash))
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
		// Delete blocks
		_, err := configs.BlocksCollections.DeleteMany(sessCtx, filter)
		if err != nil {
			return nil, err
		}

		// Update sync state
		err = StoreLastKnownBlockNumber(blockNumber)
		if err != nil {
			return nil, err
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
