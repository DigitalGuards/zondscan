package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/validation"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func InsertManyCoinbase(doc []interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := configs.CoinbaseCollections.InsertMany(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insertMany in the coinbase collection: ", zap.Error(err))
	}
}

func InsertCoinbaseDocument(blockHash string, blockNumber uint64, from string, hash string, nonce uint64, transactionIndex uint64, blockproposerReward uint64, attestorReward uint64, feeReward uint64, txType uint8, chainId uint8, signature string, pk string) (*mongo.InsertOneResult, error) {
	// Normalize address to canonical Z-prefix form
	from = validation.ConvertToZAddress(from)

	doc := primitive.D{
		{Key: "blockhash", Value: blockHash},
		{Key: "blocknumber", Value: blockNumber},
		{Key: "from", Value: from},
		{Key: "hash", Value: hash},
		{Key: "nonce", Value: nonce},
		{Key: "transactionindex", Value: transactionIndex},
		{Key: "blockproposerreward", Value: blockproposerReward},
		{Key: "attestorreward", Value: attestorReward},
		{Key: "feereward", Value: feeReward},
		{Key: "type", Value: txType},
		{Key: "chainid", Value: chainId},
		{Key: "signature", Value: signature},
		{Key: "pk", Value: pk},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := configs.CoinbaseCollections.InsertOne(ctx, doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the coinbase collection: ", zap.Error(err))
	}

	return result, err
}
