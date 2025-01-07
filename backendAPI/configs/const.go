package configs

import (
	"os"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
)

const QUANTA float64 = 1000000000000000000

var Url string = os.Getenv("NODE_URL")
var TransferCollections *mongo.Collection = GetCollection(DB, "transfer")
var TransactionByAddressCollection *mongo.Collection = GetCollection(DB, "transactionByAddress")
var InternalTransactionByAddressCollection *mongo.Collection = GetCollection(DB, "internalTransactionByAddress")
var AddressesCollection *mongo.Collection = GetCollection(DB, "addresses")
var BlocksCollection *mongo.Collection = GetCollection(DB, "blocks")
var ValidatorsCollections *mongo.Collection = GetCollection(DB, "validators")
var CoinbaseCollection *mongo.Collection = GetCollection(DB, "coinbase")
var ContractInfoCollection *mongo.Collection = GetCollection(DB, "contractCode")
var BlockSizesCollection *mongo.Collection = GetCollection(DB, "averageBlockSize")
var TotalCirculatingSupplyCollection *mongo.Collection = GetCollection(DB, "totalCirculatingSupply")
var CoinGeckoCollection *mongo.Collection = GetCollection(DB, "coingecko")
var WalletCountCollection *mongo.Collection = GetCollection(DB, "walletCount")
var DailyTransactionsVolumeCollection *mongo.Collection = GetCollection(DB, "dailyTransactionsVolume")
var Validate = validator.New()
