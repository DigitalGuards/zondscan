package db

import (
	"Zond2mongoDB/configs"
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func InsertManyCoinbase(doc []interface{}) {
	_, err := configs.CoinbaseCollections.InsertMany(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insertMany in the coinbase collection: ", zap.Error(err))
	}
}

func InsertCoinbaseDocument(blockHash string, blockNumber uint64, from string, hash string, nonce uint64, transactionIndex uint64, blockproposerReward uint64, attestorReward uint64, feeReward uint64, txType uint8, chainId uint8, signature string, pk string) (*mongo.InsertOneResult, error) {
	// Normalize address to lowercase for consistent storage
	from = strings.ToLower(from)

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

	result, err := configs.CoinbaseCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the coinbase collection: ", zap.Error(err))
	}

	return result, err
}
