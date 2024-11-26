package db

import (
	"QRLtoMongoDB-PoS/configs"
	"context"

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

func InsertCoinbaseDocument(blockHash []byte, blockNumber uint64, from []byte, hash []byte, nonce uint64, transactionIndex uint64, blockproposerReward uint64, attestorReward uint64, feeReward uint64, txType uint8, chainId uint8, signature []byte, pk []byte) (*mongo.InsertOneResult, error) {
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
