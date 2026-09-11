package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const completedBlockIngestionState = "complete"
const blockHeightUniqueIndexName = "blockNumberInt_desc_idx"

var validBlockIngestionStates = primitive.A{"pending", completedBlockIngestionState}

func completedBlockFilter(fields primitive.D) primitive.D {
	filter := make(primitive.D, 0, len(fields)+1)
	filter = append(filter, fields...)
	return append(filter, primitive.E{
		Key:   "ingestionState",
		Value: completedBlockIngestionState,
	})
}

func blockIngestionReadinessFilter() primitive.D {
	return primitive.D{{
		Key: "ingestionState", Value: primitive.D{{
			Key: "$nin", Value: validBlockIngestionStates,
		}},
	}}
}

// ValidateBlockIngestionReadiness rejects markerless or unknown block rows.
// Markerless legacy rows cannot prove that companion writes completed and
// require the explicit companion reindex migration before this backend serves.
func ValidateBlockIngestionReadiness(ctx context.Context) error {
	indexSpecifications, err := configs.BlocksCollection.Indexes().ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("validate unique canonical block index: %w", err)
	}
	if !hasUniqueBlockHeightIndex(indexSpecifications) {
		return fmt.Errorf("block ingestion migration required: unique block-height index %s is missing",
			blockHeightUniqueIndexName)
	}
	err = configs.BlocksCollection.FindOne(
		ctx,
		blockIngestionReadinessFilter(),
		options.FindOne().SetProjection(primitive.D{{Key: "_id", Value: 1}}),
	).Err()
	if err == nil {
		return fmt.Errorf("block ingestion migration required: markerless or unknown-state block rows exist")
	}
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("validate block ingestion migration: %w", err)
	}
	return nil
}

func hasUniqueBlockHeightIndex(specifications []*mongo.IndexSpecification) bool {
	for _, specification := range specifications {
		if specification != nil && specification.Name == blockHeightUniqueIndexName &&
			specification.Unique != nil && *specification.Unique {
			return true
		}
	}
	return false
}

// PingDatabase verifies the MongoDB connection is reachable, using the
// supplied context for the deadline. Used by the /health readiness probe so
// route handlers don't reach into the configs package directly.
func PingDatabase(ctx context.Context) error {
	if configs.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := configs.DB.Ping(ctx, nil); err != nil {
		return err
	}
	return ValidateBlockIngestionReadiness(ctx)
}

func ReturnSingleBlock(block uint64) (models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	// Primary lookup: use blockNumberInt (int64) for an exact numeric match.
	// This is reliable regardless of whether the hex string was stored with or
	// without zero-padding, and uses the blockNumberInt_desc_idx index.
	filter := completedBlockFilter(primitive.D{{Key: "blockNumberInt", Value: int64(block)}})
	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		// Fallback for documents written before blockNumberInt was added.
		hexBlock := fmt.Sprintf("0x%x", block)
		filter = completedBlockFilter(primitive.D{{Key: "result.number", Value: hexBlock}})
		err = configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
		if err != nil {
			return result, fmt.Errorf("block %d not found", block)
		}
	}

	return result, nil
}

// GetLatestBlockFromSyncState returns the latest block number from the sync_state collection
func GetLatestBlockFromSyncState() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result struct {
		BlockNumber string `bson:"block_number"`
	}

	err := configs.SyncStateCollection.FindOne(ctx, primitive.D{{Key: "_id", Value: "last_synced_block"}}).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("failed to get sync state: %v", err)
	}

	return result.BlockNumber, nil
}

func ReturnLatestBlocks(page int, limit int) ([]models.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var blocks []models.Result
	defer cancel()

	if limit <= 0 {
		limit = 5 // Default to 5 blocks per page
	}

	projection := primitive.D{
		{Key: "result.number", Value: 1},
		{Key: "result.timestamp", Value: 1},
		{Key: "result.hash", Value: 1},
		{Key: "result.transactions", Value: 1},
	}

	// Sort by blockNumberInt (numeric), NOT result.timestamp (hex string).
	// Hex-string sort goes wrong at width boundaries (e.g. "0x6a019244" vs
	// "0x70000000"), which would reorder the public Latest Blocks list and
	// cause pagination to duplicate/drop rows. Block number monotonically
	// tracks chain order; the blockNumberInt_desc_idx covers this sort.
	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "blockNumberInt", Value: -1}})

	if page == 0 {
		page = 1
	}
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	results, err := configs.BlocksCollection.Find(ctx, completedBlockFilter(nil), opts)
	if err != nil {
		return nil, err
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleBlock models.ZondUint64Version
		if err = results.Decode(&singleBlock); err != nil {
			continue
		}
		blocks = append(blocks, singleBlock.Result)
	}

	return blocks, nil
}

// CountBlocksNetwork returns the number of blocks whose companion rows reached
// the durable completion marker. EstimatedDocumentCount cannot apply this
// visibility fence, so this query deliberately uses an exact indexed count.
func CountBlocksNetwork() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.BlocksCollection.CountDocuments(ctx, completedBlockFilter(nil))
	if err != nil {
		return 0, err
	}

	return count, nil
}

func ReturnBlockSizes() ([]primitive.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cap the result set: averageBlockSize grows without bound as the syncer
	// appends time-series rows, and this feeds a chart that only renders a
	// finite window. 2000 points is well past what any chart shows.
	opts := options.Find().
		SetSort(primitive.D{{Key: "timestamp", Value: 1}}).
		SetLimit(2000)

	cursor, err := configs.BlockSizesCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query block sizes: %w", err)
	}

	var episodes []primitive.M
	if err = cursor.All(ctx, &episodes); err != nil {
		log.Printf("error decoding block sizes: %v", err)
	}

	return episodes, err
}
