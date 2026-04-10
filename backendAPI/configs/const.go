package configs

import (
	"go.mongodb.org/mongo-driver/mongo"
)

var TransferCollections *mongo.Collection = GetCollection(DB, "transfer")
var TransactionByAddressCollection *mongo.Collection = GetCollection(DB, "transactionByAddress")
var InternalTransactionByAddressCollection *mongo.Collection = GetCollection(DB, "internalTransactionByAddress")
var AddressesCollections *mongo.Collection = GetCollection(DB, "addresses")
var BlocksCollection *mongo.Collection = GetCollection(DB, "blocks")
var ValidatorsCollections *mongo.Collection = GetCollection(DB, "validators")
var ContractInfoCollection *mongo.Collection = GetCollection(DB, "contractCode")
var BlockSizesCollection *mongo.Collection = GetCollection(DB, "averageBlockSize")
var TotalCirculatingSupplyCollection *mongo.Collection = GetCollection(DB, "totalCirculatingSupply")
var CoinGeckoCollection *mongo.Collection = GetCollection(DB, "coingecko")
var WalletCountCollections *mongo.Collection = GetCollection(DB, "walletCount")
var DailyTransactionsVolumeCollection *mongo.Collection = GetCollection(DB, "dailyTransactionsVolume")
var EpochInfoCollection *mongo.Collection = GetCollection(DB, "epoch_info")
var ValidatorHistoryCollection *mongo.Collection = GetCollection(DB, "validator_history")
var PriceHistoryCollection *mongo.Collection = GetCollection(DB, "priceHistory")
