package db

import (
	"QRLtoMongoDB-PoS/configs"
	"context"

	"go.mongodb.org/mongo-driver/bson"
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
	doc := bson.D{{"blockhash", blockHash}, {"blocknumber", blockNumber}, {"from", from}, {"hash", hash}, {"nonce", nonce}, {"transactionindex", transactionIndex}, {"blockproposerreward", blockproposerReward}, {"attestorreward", attestorReward}, {"feereward", feeReward}, {"type", txType}, {"chainid", chainId}, {"signature", signature}, {"pk", pk}}
	result, err := configs.CoinbaseCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the coinbase collection: ", zap.Error(err))
	}

	return result, err
}
