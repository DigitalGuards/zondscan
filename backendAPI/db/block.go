package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnSingleBlock(block uint64) (models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	// Convert decimal block number to hex format with 0x prefix
	hexBlock := fmt.Sprintf("0x%x", block)
	filter := primitive.D{{Key: "result.number", Value: hexBlock}}

	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		// Try with zero-padded hex if the first attempt failed
		hexBlockPadded := fmt.Sprintf("0x%02x", block)
		if hexBlockPadded != hexBlock {
			filter = primitive.D{{Key: "result.number", Value: hexBlockPadded}}
			err = configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
		}
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

	err := configs.GetCollection(configs.DB, "sync_state").FindOne(ctx, primitive.D{{Key: "_id", Value: "last_synced_block"}}).Decode(&result)
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

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "result.timestamp", Value: -1}})

	if page == 0 {
		page = 1
	}
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	results, err := configs.BlocksCollection.Find(ctx, primitive.D{}, opts)
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

func CountBlocksNetwork() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.BlocksCollection.CountDocuments(ctx, primitive.D{})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func ReturnHashToBlockNumber(query string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	filter := primitive.D{{Key: "result.hash", Value: query}}
	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to find block: %v", err)
	}

	// Convert hex string to uint64
	numStr := result.Result.Number
	if strings.HasPrefix(numStr, "0x") {
		numStr = numStr[2:] // Remove "0x" prefix
		num, err := strconv.ParseUint(numStr, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse block number: %v", err)
		}
		return num, nil
	}
	return 0, fmt.Errorf("invalid block number format: %s", numStr)
}

func ReturnBlockSizes() ([]primitive.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(primitive.D{{Key: "timestamp", Value: 1}})

	cursor, err := configs.BlockSizesCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		panic(err)
	}

	var episodes []primitive.M
	if err = cursor.All(ctx, &episodes); err != nil {
		fmt.Println(err)
	}

	return episodes, err
}
