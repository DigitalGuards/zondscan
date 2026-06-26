package configs

import (
	"os"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
)

const QUANTA float64 = 1000000000000000000

var Url string = os.Getenv("NODE_URL")

// Collection handles. These are populated by ConnectDB once the client is
// confirmed live (see initCollections), not at package-init time. Binding
// them at declaration via GetCollection(DB, ...) used to force a Mongo
// connect during package initialization (because DB is nil there, the
// nil-guard in GetCollection calls ConnectDB), which meant a Mongo outage
// log.Fatal'd before main's panic recovery was installed and left the
// startup ordering fragile. Every reader of these vars runs inside an HTTP
// handler, which only executes after RequestHandler has called ConnectDB.
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
)

var Validate = validator.New()
