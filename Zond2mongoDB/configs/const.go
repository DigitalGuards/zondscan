package configs

import (
	L "Zond2mongoDB/logger"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const QUANTA float64 = 1000000000000000000

// QRL address constants
const QRLZeroAddress = "Z0000000000000000000000000000000000000000"

// Collection names
const (
	ADDRESSES_COLLECTION                       = "addresses"
	BLOCKS_COLLECTION                          = "blocks"
	COINBASE_COLLECTION                        = "coinbase"
	INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION = "internalTransactionByAddress"
	TRANSACTION_BY_ADDRESS_COLLECTION          = "transactionByAddress"
	TRANSFER_COLLECTION                        = "transfer"
	ATTESTOR_COLLECTION                        = "attestor"
	STAKE_COLLECTION                           = "stake"
	VALIDATORS_COLLECTION                      = "validators"
	CONTRACT_CODE_COLLECTION                   = "contractCode"
	AVERAGE_BLOCK_SIZE_COLLECTION              = "averageBlockSize"
	TOTAL_CIRCULATING_QUANTA_COLLECTION        = "totalCirculatingQuanta"
	COINGECKO_COLLECTION                       = "coingecko"
	WALLET_COUNT_COLLECTION                    = "walletCount"
	DAILY_TRANSACTIONS_VOLUME_COLLECTION       = "dailyTransactionsVolume"
	PENDING_TRANSACTIONS_COLLECTION            = "pending_transactions"
	EPOCH_INFO_COLLECTION                      = "epoch_info"
	VALIDATOR_HISTORY_COLLECTION               = "validator_history"
)

// API and configuration constants
var COINGECKO_URL string = "https://api.coingecko.com/api/v3/coins/quantum-resistant-ledger?tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false"

// Logging configuration
const LOG_FILENAME = "zond_sync.log"

// MongoDB collections
var AddressesCollections *mongo.Collection = GetCollection(DB, ADDRESSES_COLLECTION)
var BlocksCollections *mongo.Collection = GetCollection(DB, BLOCKS_COLLECTION)
var CoinbaseCollections *mongo.Collection = GetCollection(DB, COINBASE_COLLECTION)
var InternalTransactionByAddressCollections *mongo.Collection = GetCollection(DB, INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION)
var TransactionByAddressCollections *mongo.Collection = GetCollection(DB, TRANSACTION_BY_ADDRESS_COLLECTION)
var TransferCollections *mongo.Collection = GetCollection(DB, TRANSFER_COLLECTION)
var AttestorCollections *mongo.Collection = GetCollection(DB, ATTESTOR_COLLECTION)
var StakeCollections *mongo.Collection = GetCollection(DB, STAKE_COLLECTION)
var ValidatorsCollections *mongo.Collection = GetCollection(DB, VALIDATORS_COLLECTION)
var ContractCodeCollection *mongo.Collection = GetCollection(DB, CONTRACT_CODE_COLLECTION)
var AverageBlockSizeCollections *mongo.Collection = GetCollection(DB, AVERAGE_BLOCK_SIZE_COLLECTION)
var TotalCirculatingQuantaCollections *mongo.Collection = GetCollection(DB, TOTAL_CIRCULATING_QUANTA_COLLECTION)
var CoinGeckoCollections *mongo.Collection = GetCollection(DB, COINGECKO_COLLECTION)
var WalletCountCollections *mongo.Collection = GetCollection(DB, WALLET_COUNT_COLLECTION)
var DailyTransactionsVolumeCollections *mongo.Collection = GetCollection(DB, DAILY_TRANSACTIONS_VOLUME_COLLECTION)
var PendingTransactionsCollections *mongo.Collection = GetCollection(DB, PENDING_TRANSACTIONS_COLLECTION)
var EpochInfoCollections *mongo.Collection = GetCollection(DB, EPOCH_INFO_COLLECTION)
var ValidatorHistoryCollections *mongo.Collection = GetCollection(DB, VALIDATOR_HISTORY_COLLECTION)

// Global logger instance - initialized once and used throughout the application
var Logger *zap.Logger = L.FileLogger(LOG_FILENAME)
