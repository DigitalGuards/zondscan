package configs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func ConnectDB() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI(EnvMongoURI()))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	//ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB")

	// Initialize collections with validators
	db := client.Database("qrldata-z")

	// Daily Transactions Volume
	volumeValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"volume", "timestamp"},
			"properties": bson.M{
				"volume": bson.M{
					"bsonType":    "double",
					"description": "transaction volume in QRL",
				},
				"timestamp": bson.M{
					"bsonType":    "string",
					"description": "block timestamp as hex string",
				},
				"transferCount": bson.M{
					"bsonType":    "int",
					"description": "number of transfers in the period",
				},
			},
		},
	}
	ensureCollection(db, "dailyTransactionsVolume", volumeValidator)

	// CoinGecko Data (current price)
	coingeckoValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"marketCapUSD", "priceUSD", "lastUpdated"},
			"properties": bson.M{
				"marketCapUSD": bson.M{
					"bsonType":    "double",
					"description": "must be a double and is required",
				},
				"priceUSD": bson.M{
					"bsonType":    "double",
					"description": "must be a double and is required",
				},
				"volumeUSD": bson.M{
					"bsonType":    "double",
					"description": "24h trading volume in USD",
				},
				"lastUpdated": bson.M{
					"bsonType":    "date",
					"description": "must be a date and is required",
				},
			},
		},
	}
	ensureCollection(db, "coingecko", coingeckoValidator)

	// Price History (historical snapshots)
	priceHistoryValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"timestamp", "priceUSD"},
			"properties": bson.M{
				"timestamp": bson.M{
					"bsonType":    "date",
					"description": "must be a date and is required",
				},
				"priceUSD": bson.M{
					"bsonType":    "double",
					"description": "must be a double and is required",
				},
				"marketCapUSD": bson.M{
					"bsonType":    "double",
					"description": "market cap in USD",
				},
				"volumeUSD": bson.M{
					"bsonType":    "double",
					"description": "24h trading volume in USD",
				},
			},
		},
	}
	ensureCollection(db, "priceHistory", priceHistoryValidator)

	// Wallet Count
	walletValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"count"},
			"properties": bson.M{
				"count": bson.M{
					"bsonType":    "long",
					"description": "must be a long/int64 and is required",
				},
			},
		},
	}
	ensureCollection(db, "walletCount", walletValidator)

	// Total Circulating Supply
	circulatingValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"circulating"},
			"properties": bson.M{
				"circulating": bson.M{
					"bsonType":    "string",
					"description": "must be a string and is required",
				},
			},
		},
	}
	ensureCollection(db, "totalCirculatingSupply", circulatingValidator)

	// Token Balances
	//
	// Phase 2 adds optional `tokenID` (decimal uint256) and `tokenStandard`
	// (ERC-20/721/1155) so the same collection stores per-tokenID NFT
	// ownership rows alongside the legacy ERC-20 ones. Both are omitted on
	// ERC-20 rows so the validator keeps them optional.
	tokenBalanceValidator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": []string{"contractAddress", "holderAddress", "balance", "blockNumber", "updatedAt"},
			"properties": bson.M{
				"contractAddress": bson.M{
					"bsonType":    "string",
					"description": "must be a hex string and is required",
				},
				"holderAddress": bson.M{
					"bsonType":    "string",
					"description": "must be a hex string and is required",
				},
				"balance": bson.M{
					"bsonType":    "string",
					"description": "must be a hex string and is required",
				},
				"blockNumber": bson.M{
					"bsonType":    "string",
					"description": "must be a hex string and is required",
				},
				"updatedAt": bson.M{
					"bsonType":    "string",
					"description": "must be a string and is required",
				},
				"tokenID": bson.M{
					"bsonType":    "string",
					"description": "decimal uint256 token id, optional (NFTs only)",
				},
				"tokenStandard": bson.M{
					"bsonType":    "string",
					"description": "ERC-20 | ERC-721 | ERC-1155, optional",
				},
			},
		},
	}
	ensureCollection(db, "tokenBalances", tokenBalanceValidator)

	// Initialize collections
	initializeCollections(db)

	// Initialize sync state collection
	_, err = db.Collection("sync_state").UpdateOne(
		ctx,
		bson.M{"_id": "last_synced_block"},
		bson.M{"$setOnInsert": bson.M{"block_number": "0x0"}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		Logger.Error("Failed to initialize sync state collection", zap.Error(err))
	}

	return client
}

func ensureCollection(db *mongo.Database, name string, validator bson.M) {
	cmd := bson.D{
		{Key: "collMod", Value: name},
		{Key: "validator", Value: validator},
		{Key: "validationLevel", Value: "strict"},
	}

	err := db.RunCommand(context.Background(), cmd).Err()
	if err != nil {
		// If collection doesn't exist, create it with the validator
		if err.Error() == "not found" {
			opts := options.CreateCollection().SetValidator(validator)
			err = db.CreateCollection(context.Background(), name, opts)
			if err != nil {
				Logger.Warn("Could not create collection with validator",
					zap.String("collection", name),
					zap.Error(err))
			} else {
				Logger.Info("Created collection with validator",
					zap.String("collection", name))
			}
		} else {
			Logger.Warn("Could not set up validator",
				zap.String("collection", name),
				zap.Error(err))
		}
	} else {
		Logger.Info("Updated validator for collection",
			zap.String("collection", name))
	}
}

func initializeCollections(db *mongo.Database) {
	// Bound every index/collection setup call inside this function so a
	// stalled MongoDB can't hang syncer startup indefinitely. 60s is
	// generous enough for any single CreateIndex over this dataset; if a
	// build truly takes longer the operation is too big for startup and
	// should be moved offline.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// tokenBalances indexes are owned by db.InitializeTokenBalancesCollection
	// (called from synchroniser/token_sync.go at syncer startup). Keeping the
	// owner in one place lets the Phase 2 migration drop the legacy 2-tuple
	// unique and replace it with the 3-tuple `(contract, holder, tokenID)`
	// without a second creator racing it on each restart.

	// Initialize pending token contracts collection with compound index
	pendingTokenContractsCollection := db.Collection("pending_token_contracts")
	_, err := pendingTokenContractsCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "txHash", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	)
	if err != nil {
		// A previously-created index over the same (contractAddress, txHash)
		// keys under a different name (e.g. the named "contract_tx_idx" set
		// by db.InitializePendingTokenContractsCollection) makes Mongo reject
		// this auto-named duplicate with IndexOptionsConflict. That is benign:
		// the index already exists, so do not treat it as an error.
		msg := err.Error()
		if strings.Contains(msg, "IndexOptionsConflict") ||
			strings.Contains(msg, "IndexKeySpecsConflict") ||
			strings.Contains(msg, "already exists") {
			Logger.Info("Pending token contracts unique index already exists, skipping")
		} else {
			Logger.Error("Failed to create index for pending token contracts collection", zap.Error(err))
		}
	}

	// Also add index on the processed field for efficient querying
	_, err = pendingTokenContractsCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "processed", Value: 1}},
			Options: options.Index().SetName("processed_idx"),
		},
	)
	if err != nil {
		Logger.Error("Failed to create processed index for pending token contracts collection", zap.Error(err))
	}

	// tokenTransfers indexes are owned by db.InitializeTokenTransfersCollection
	// (called from synchroniser.InitializeTokenCollections at startup). Creating
	// a duplicate, auto-named set here causes IndexOptionsConflict against the
	// named set declared in db/tokentransfers.go, which aborts the entire
	// CreateMany in the proper init — including the (txHash, contract, logIndex,
	// tokenID) unique that ERC-1155 TransferBatch + ERC-721 batch mint depend on
	// to land more than one row per tx. Leaving that single source of truth in
	// place here keeps the indexes aligned with the model that the writer code
	// actually expects.

	// Initialize CoinGecko collection with empty document
	_, err = db.Collection("coingecko").UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{
			"marketCapUSD": 0.0,
			"priceUSD":     0.0,
			"volumeUSD":    0.0,
			"lastUpdated":  time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		Logger.Error("Failed to initialize CoinGecko collection", zap.Error(err))
	}

	// Initialize priceHistory collection with timestamp index for efficient time-range queries
	priceHistoryCollection := db.Collection("priceHistory")
	_, err = priceHistoryCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "timestamp", Value: -1}}, // Descending for recent-first queries
			Options: options.Index().SetName("timestamp_desc_idx"),
		},
	)
	if err != nil {
		Logger.Error("Failed to create index for priceHistory collection", zap.Error(err))
	} else {
		Logger.Info("Price history collection initialized with timestamp index")
	}

	// Initialize transfer collection with blockTimestamp index for efficient time-range queries (daily volume)
	transferCollection := db.Collection("transfer")
	_, err = transferCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "blockTimestamp", Value: -1}}, // Descending for recent-first queries
			Options: options.Index().SetName("blockTimestamp_desc_idx"),
		},
	)
	if err != nil {
		Logger.Error("Failed to create blockTimestamp index for transfer collection", zap.Error(err))
	} else {
		Logger.Info("Transfer collection initialized with blockTimestamp index")
	}

	// Create and set up the rest of the collections
	ensureCollection(db, "blocks", nil)

	// Add index on blockNumberInt for efficient numeric range queries on blocks.
	// This replaces the old pattern of doing hex string $gte/$lte which produced
	// incorrect lexicographic ordering (e.g. "0x9" > "0x10").
	blocksCollection := db.Collection("blocks")
	_, err = blocksCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "blockNumberInt", Value: -1}},
			Options: options.Index().SetName("blockNumberInt_desc_idx"),
		},
	)
	if err != nil {
		Logger.Error("Failed to create blockNumberInt index for blocks collection", zap.Error(err))
	} else {
		Logger.Info("Blocks collection initialized with blockNumberInt index")
	}

	ensureCollection(db, "validators", nil)
	ensureCollection(db, "contractCode", nil)
	ensureCollection(db, "transactionByAddress", nil)
	ensureCollection(db, "internalTransactionByAddress", nil)
	ensureCollection(db, "contracts", nil)
	ensureCollection(db, "addresses", nil)

	// Create unique index on addresses.id to prevent duplicate entries from concurrent upserts
	addressesCollection := db.Collection("addresses")
	_, err = addressesCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetName("addresses_id_unique_idx").SetUnique(true),
		},
	)
	if err != nil {
		Logger.Warn("Could not create unique index on addresses.id (duplicates may exist, run dedup)", zap.Error(err))
	} else {
		Logger.Info("Addresses collection initialized with unique id index")
	}
	ensureCollection(db, "walletCount", nil)
	ensureCollection(db, "dailyTransactionsVolume", nil)
	ensureCollection(db, "totalCirculatingSupply", nil)
	ensureCollection(db, "sync_state", nil)

	// Create indexes on the validators collection for per-document lookup.
	validatorsCollection := db.Collection("validators")
	_, err = validatorsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "publicKeyHex", Value: 1}},
			Options: options.Index().SetName("validators_pubkey_idx"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("validators_status_idx"),
		},
		{
			Keys:    bson.D{{Key: "effectiveBalance", Value: -1}},
			Options: options.Index().SetName("validators_balance_desc_idx"),
		},
	})
	if err != nil {
		Logger.Warn("Could not create validators collection indexes", zap.Error(err))
	}

	// Indexes on pending_transactions. The backend paginates by {status, createdAt}
	// (`/pending-transactions` and the homepage poll) and the syncer cleans by
	// lastSeen (`CleanupOldPendingTransactions`); both were doing collection scans
	// before. Compound + single keys here are sized for the access pattern, not
	// for general flexibility.
	pendingTransactionsCollection := db.Collection(PENDING_TRANSACTIONS_COLLECTION)
	_, err = pendingTransactionsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("pending_status_createdAt_idx"),
		},
		{
			Keys:    bson.D{{Key: "lastSeen", Value: 1}},
			Options: options.Index().SetName("pending_lastSeen_idx"),
		},
	})
	if err != nil {
		Logger.Warn("Could not create pending_transactions indexes", zap.Error(err))
	}

	// validator_history sorts by epochInt -1 on the public /epochs listing.
	validatorHistoryCollection := db.Collection(VALIDATOR_HISTORY_COLLECTION)
	_, err = validatorHistoryCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "epochInt", Value: -1}},
		Options: options.Index().SetName("validator_history_epochInt_desc_idx"),
	})
	if err != nil {
		Logger.Warn("Could not create validator_history.epochInt index", zap.Error(err))
	}

	Logger.Info("All collections initialized successfully")
}

// Client instance
var DB *mongo.Client = ConnectDB()

// Getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	collection := client.Database("qrldata-z").Collection(collectionName)
	return collection
}

// Getter for contracts collection
func GetContractsCollection() *mongo.Collection {
	return GetCollection(DB, CONTRACT_CODE_COLLECTION)
}

// Getter for validator collection
func GetValidatorCollection() *mongo.Collection {
	return GetCollection(DB, VALIDATORS_COLLECTION)
}

// Getter for token balances collection
func GetTokenBalancesCollection() *mongo.Collection {
	return GetCollection(DB, "tokenBalances")
}

// GetTokenTransfersCollection returns the tokenTransfers collection
func GetTokenTransfersCollection() *mongo.Collection {
	// Use GetCollection with explicit collection name
	coll := GetCollection(DB, "tokenTransfers")

	// Log that we're getting a reference to the collection
	Logger.Debug("Getting tokenTransfers collection reference")

	return coll
}

func GetListCollectionNames(client *mongo.Client) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.Database("qrldata-z").ListCollectionNames(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}

	return result
}
