package configs

import (
	L "QRL2MongoDB/logger"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// QRL address constants
const QRLZeroAddress = "Q" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000"

// Collection names
const (
	ADDRESSES_COLLECTION                       = "addresses"
	BLOCKS_COLLECTION                          = "blocks"
	INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION = "internalTransactionByAddress"
	TRANSACTION_BY_ADDRESS_COLLECTION          = "transactionByAddress"
	TRANSFER_COLLECTION                        = "transfer"
	VALIDATORS_COLLECTION                      = "validators"
	CONTRACT_CODE_COLLECTION                   = "contractCode"
	CONTRACT_VERIFICATIONS_COLLECTION          = "contractVerifications"
	AVERAGE_BLOCK_SIZE_COLLECTION              = "averageBlockSize"
	COINGECKO_COLLECTION                       = "coingecko"
	WALLET_COUNT_COLLECTION                    = "walletCount"
	DAILY_TRANSACTIONS_VOLUME_COLLECTION       = "dailyTransactionsVolume"
	PENDING_TRANSACTIONS_COLLECTION            = "pending_transactions"
	EPOCH_INFO_COLLECTION                      = "epoch_info"
	VALIDATOR_HISTORY_COLLECTION               = "validator_history"
	PRICE_HISTORY_COLLECTION                   = "priceHistory"
	SYNCER_LEASES_COLLECTION                   = "syncer_leases"
	BALANCE_RECONCILIATIONS_COLLECTION         = "balance_reconciliations"
	TOKEN_EVENT_DEAD_LETTERS_COLLECTION        = "tokenEventDeadLetters"
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
	InternalTransactionByAddressCollections *mongo.Collection
	TransactionByAddressCollections         *mongo.Collection
	TransferCollections                     *mongo.Collection
	ValidatorsCollections                   *mongo.Collection
	AverageBlockSizeCollections             *mongo.Collection
	CoinGeckoCollections                    *mongo.Collection
	WalletCountCollections                  *mongo.Collection
	DailyTransactionsVolumeCollections      *mongo.Collection
	PendingTransactionsCollections          *mongo.Collection
	EpochInfoCollections                    *mongo.Collection
	ValidatorHistoryCollections             *mongo.Collection
	PriceHistoryCollections                 *mongo.Collection
	SyncerLeasesCollection                  *mongo.Collection
	TokenEventDeadLettersCollection         *mongo.Collection
)

// bindCollections populates the package-level collection handles above.
// Called by ConnectDB after the client is connected and pinged.
func bindCollections(client *mongo.Client) {
	AddressesCollections = GetCollection(client, ADDRESSES_COLLECTION)
	BlocksCollections = GetCollection(client, BLOCKS_COLLECTION)
	InternalTransactionByAddressCollections = GetCollection(client, INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION)
	TransactionByAddressCollections = GetCollection(client, TRANSACTION_BY_ADDRESS_COLLECTION)
	TransferCollections = GetCollection(client, TRANSFER_COLLECTION)
	ValidatorsCollections = GetCollection(client, VALIDATORS_COLLECTION)
	AverageBlockSizeCollections = GetCollection(client, AVERAGE_BLOCK_SIZE_COLLECTION)
	CoinGeckoCollections = GetCollection(client, COINGECKO_COLLECTION)
	WalletCountCollections = GetCollection(client, WALLET_COUNT_COLLECTION)
	DailyTransactionsVolumeCollections = GetCollection(client, DAILY_TRANSACTIONS_VOLUME_COLLECTION)
	PendingTransactionsCollections = GetCollection(client, PENDING_TRANSACTIONS_COLLECTION)
	EpochInfoCollections = GetCollection(client, EPOCH_INFO_COLLECTION)
	ValidatorHistoryCollections = GetCollection(client, VALIDATOR_HISTORY_COLLECTION)
	PriceHistoryCollections = GetCollection(client, PRICE_HISTORY_COLLECTION)
	SyncerLeasesCollection = GetCollection(client, SYNCER_LEASES_COLLECTION)
	TokenEventDeadLettersCollection = GetCollection(client, TOKEN_EVENT_DEAD_LETTERS_COLLECTION)
}

// Global logger instance - initialized once and used throughout the application
var Logger *zap.Logger = L.FileLogger(LOG_FILENAME)
