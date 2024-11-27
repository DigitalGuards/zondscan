package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnSingleBlock(block uint64) (models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	filter := primitive.D{{Key: "result.number", Value: block}}
	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnLatestBlock() ([]models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var blocks []models.ZondUint64Version
	defer cancel()

	projection := primitive.D{
		{Key: "result.number", Value: 1},
		{Key: "result.timestamp", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "result.number", Value: -1}}).
		SetLimit(1)

	results, err := configs.BlocksCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleBlock models.ZondUint64Version
		if err = results.Decode(&singleBlock); err != nil {
			fmt.Println(err)
		}
		blocks = append(blocks, singleBlock)
	}

	return blocks, nil
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
		fmt.Println(err)
	}

	return result.Result.Number, err
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
