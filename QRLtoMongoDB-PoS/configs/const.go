package configs

import (
	L "QRLtoMongoDB-PoS/logger"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// http://209.250.255.226:8545

// sudo mongod --dbpath /System/Volumes/Data/data/db
var QUANTA float32 = 1000000000000000000
var COINGECKO_URL string = "https://api.coingecko.com/api/v3/coins/quantum-resistant-ledger?tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false"
var Filename = "logs.log"
var AddressesCollections *mongo.Collection = GetCollection(DB, "addresses")
var BlocksCollections *mongo.Collection = GetCollection(DB, "blocks")
var CoinbaseCollections *mongo.Collection = GetCollection(DB, "coinbase")
var InternalTransactionByAddressCollections *mongo.Collection = GetCollection(DB, "internalTransactionByAddress")
var TransactionByAddressCollections *mongo.Collection = GetCollection(DB, "transactionByAddress")
var TransferCollections *mongo.Collection = GetCollection(DB, "transfer")
var AttestorCollections *mongo.Collection = GetCollection(DB, "attestor")
var StakeCollections *mongo.Collection = GetCollection(DB, "stake")
var ValidatorsCollections *mongo.Collection = GetCollection(DB, "validators")
var BitfieldCollections *mongo.Collection = GetCollection(DB, "bitfield")
var ContractCodeCollections *mongo.Collection = GetCollection(DB, "contractCode")
var AverageBlockSizeCollections *mongo.Collection = GetCollection(DB, "averageBlockSize")
var TotalCirculatingQuantaCollections *mongo.Collection = GetCollection(DB, "totalCirculatingQuanta")
var CoinGeckoCollections *mongo.Collection = GetCollection(DB, "coingecko")
var WalletCountCollections *mongo.Collection = GetCollection(DB, "walletCount")
var DailyTransactionsVolumeCollections *mongo.Collection = GetCollection(DB, "dailyTransactionsVolume")
var Logger *zap.Logger = L.FileLogger(Filename)
