package configs

import (
	"os"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
)

const QUANTA float64 = 1000000000000000000

var Url string = os.Getenv("NODE_URL")

// Mongo collection names (database qrldata-z). Single source of truth for
// the literals: bindCollections binds the handles below to these names,
// and setup.go's index/seed maps reference them, so a rename can't drift
// between the two.
const (
	transferCollName                     = "transfer"
	transactionByAddressCollName         = "transactionByAddress"
	internalTransactionByAddressCollName = "internalTransactionByAddress"
	addressesCollName                    = "addresses"
	blocksCollName                       = "blocks"
	validatorsCollName                   = "validators"
	contractCodeCollName                 = "contractCode"
	contractVerificationsCollName        = "contractVerifications"
	blockSizesCollName                   = "averageBlockSize"
	totalCirculatingSupplyCollName       = "totalCirculatingSupply"
	coinGeckoCollName                    = "coingecko"
	walletCountCollName                  = "walletCount"
	dailyTransactionsVolumeCollName      = "dailyTransactionsVolume"
	epochInfoCollName                    = "epoch_info"
	validatorHistoryCollName             = "validator_history"
	priceHistoryCollName                 = "priceHistory"
	tokenTransfersCollName               = "tokenTransfers"
	tokenBalancesCollName                = "tokenBalances"
	pendingTransactionsCollName          = "pending_transactions"
	gasHistoryCollName                   = "gasHistory"
	syncStateCollName                    = "sync_state"
	tokenMetadataCollName                = "tokenMetadata"
)

// Collection handles. These are populated by ConnectDB once the client is
// confirmed live (see bindCollections), not at package-init time. Binding
// them at declaration used to force a Mongo connect during package
// initialization (the deleted per-call collection getter nil-guarded on DB
// and auto-connected), which meant a Mongo outage log.Fatal'd before
// main's panic recovery was installed and left the startup ordering
// fragile. Every reader of these vars runs inside an HTTP handler, which
// only executes after RequestHandler has called ConnectDB; until then the
// handles are nil by design.
var (
	TransferCollections                    *mongo.Collection
	TransactionByAddressCollection         *mongo.Collection
	InternalTransactionByAddressCollection *mongo.Collection
	AddressesCollections                   *mongo.Collection
	BlocksCollection                       *mongo.Collection
	ValidatorsCollections                  *mongo.Collection
	ContractInfoCollection                 *mongo.Collection
	ContractVerificationsCollection        *mongo.Collection
	BlockSizesCollection                   *mongo.Collection
	TotalCirculatingSupplyCollection       *mongo.Collection
	CoinGeckoCollection                    *mongo.Collection
	WalletCountCollections                 *mongo.Collection
	DailyTransactionsVolumeCollection      *mongo.Collection
	EpochInfoCollection                    *mongo.Collection
	ValidatorHistoryCollection             *mongo.Collection
	PriceHistoryCollection                 *mongo.Collection
	TokenTransfersCollection               *mongo.Collection
	TokenBalancesCollection                *mongo.Collection
	PendingTransactionsCollection          *mongo.Collection
	GasHistoryCollection                   *mongo.Collection
	SyncStateCollection                    *mongo.Collection
	TokenMetadataCollection                *mongo.Collection
)

var Validate = validator.New()
