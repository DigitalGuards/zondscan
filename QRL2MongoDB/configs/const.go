package configs

import (
	L "QRL2MongoDB/logger"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// QRL address constants
const QRLZeroAddress = "Q0000000000000000000000000000000000000000"

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
	PRICE_HISTORY_COLLECTION                   = "priceHistory"
)

// API and configuration constants
var COINGECKO_URL string = "https://api.coingecko.com/api/v3/coins/quantum-resistant-ledger?tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false"

// Logging configuration
const LOG_FILENAME = "zond_sync.log"

// MongoDB collection handles. Nil until ConnectDB runs bindCollections;
// nothing binds at import time so tests can import this package without a
// live MongoDB.
var (
	AddressesCollections                    *mongo.Collection
	BlocksCollections                       *mongo.Collection
	CoinbaseCollections                     *mongo.Collection
	InternalTransactionByAddressCollections *mongo.Collection
	TransactionByAddressCollections         *mongo.Collection
	TransferCollections                     *mongo.Collection
	AttestorCollections                     *mongo.Collection
	StakeCollections                        *mongo.Collection
	ValidatorsCollections                   *mongo.Collection
	ContractCodeCollection                  *mongo.Collection
	AverageBlockSizeCollections             *mongo.Collection
	TotalCirculatingQuantaCollections       *mongo.Collection
	CoinGeckoCollections                    *mongo.Collection
	WalletCountCollections                  *mongo.Collection
	DailyTransactionsVolumeCollections      *mongo.Collection
	PendingTransactionsCollections          *mongo.Collection
	EpochInfoCollections                    *mongo.Collection
	ValidatorHistoryCollections             *mongo.Collection
	PriceHistoryCollections                 *mongo.Collection
)

// bindCollections populates the package-level collection handles above.
// Called by ConnectDB after the client is connected and pinged.
func bindCollections(client *mongo.Client) {
	AddressesCollections = GetCollection(client, ADDRESSES_COLLECTION)
	BlocksCollections = GetCollection(client, BLOCKS_COLLECTION)
	CoinbaseCollections = GetCollection(client, COINBASE_COLLECTION)
	InternalTransactionByAddressCollections = GetCollection(client, INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION)
	TransactionByAddressCollections = GetCollection(client, TRANSACTION_BY_ADDRESS_COLLECTION)
	TransferCollections = GetCollection(client, TRANSFER_COLLECTION)
	AttestorCollections = GetCollection(client, ATTESTOR_COLLECTION)
	StakeCollections = GetCollection(client, STAKE_COLLECTION)
	ValidatorsCollections = GetCollection(client, VALIDATORS_COLLECTION)
	ContractCodeCollection = GetCollection(client, CONTRACT_CODE_COLLECTION)
	AverageBlockSizeCollections = GetCollection(client, AVERAGE_BLOCK_SIZE_COLLECTION)
	TotalCirculatingQuantaCollections = GetCollection(client, TOTAL_CIRCULATING_QUANTA_COLLECTION)
	CoinGeckoCollections = GetCollection(client, COINGECKO_COLLECTION)
	WalletCountCollections = GetCollection(client, WALLET_COUNT_COLLECTION)
	DailyTransactionsVolumeCollections = GetCollection(client, DAILY_TRANSACTIONS_VOLUME_COLLECTION)
	PendingTransactionsCollections = GetCollection(client, PENDING_TRANSACTIONS_COLLECTION)
	EpochInfoCollections = GetCollection(client, EPOCH_INFO_COLLECTION)
	ValidatorHistoryCollections = GetCollection(client, VALIDATOR_HISTORY_COLLECTION)
	PriceHistoryCollections = GetCollection(client, PRICE_HISTORY_COLLECTION)
}

// Global logger instance - initialized once and used throughout the application
var Logger *zap.Logger = L.FileLogger(LOG_FILENAME)
