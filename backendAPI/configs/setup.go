package configs

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Client instance
var DB *mongo.Client
var dbOnce sync.Once

// ConnectDB establishes a connection to MongoDB
// It uses a sync.Once to ensure the connection is only established once
func ConnectDB() *mongo.Client {
	dbOnce.Do(func() {
		// Conservative connection-pool defaults so a burst of concurrent HTTP
		// handlers doesn't exhaust or thrash Mongo connections. The driver's
		// zero-value pool is effectively unbounded, which lets a traffic spike
		// open an unbounded number of sockets.
		clientOpts := options.Client().
			ApplyURI(EnvMongoURI()).
			SetMaxPoolSize(100).
			SetMinPoolSize(5).
			SetMaxConnIdleTime(60 * time.Second)

		client, err := mongo.NewClient(clientOpts)
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
		log.Println("Connected to MongoDB")

		// Initialize collections with validators and indexes
		db := client.Database("qrldata-z")

		// Create indexes for collections we query
		createIndexes(db)

		// Initialize collections with fallback data if they don't exist yet
		initializeCollections(db)

		// Market trade storage is created explicitly. createIndexes above
		// skips collections that do not exist yet, so a collection first
		// written at runtime would otherwise run unindexed until some later
		// restart happened to find it populated.
		ensureMarketTradesCollection(db)

		// Set the global DB variable
		DB = client

		// Bind the package-level collection handles now that the client is
		// confirmed live. Doing this here (rather than at package-var
		// declaration time) removes the nil-client window: every reader runs
		// inside an HTTP handler, which only executes after this point.
		bindCollections(client)
	})

	return DB
}

// bindCollections wires the package-level *mongo.Collection handles to the
// live client. Called once from ConnectDB after the ping succeeds.
func bindCollections(client *mongo.Client) {
	db := client.Database("qrldata-z")
	TransferCollections = db.Collection(transferCollName)
	TransactionByAddressCollection = db.Collection(transactionByAddressCollName)
	InternalTransactionByAddressCollection = db.Collection(internalTransactionByAddressCollName)
	AddressesCollections = db.Collection(addressesCollName)
	BlocksCollection = db.Collection(blocksCollName)
	ValidatorsCollections = db.Collection(validatorsCollName)
	ContractInfoCollection = db.Collection(contractCodeCollName)
	ContractVerificationsCollection = db.Collection(contractVerificationsCollName)
	BlockSizesCollection = db.Collection(blockSizesCollName)
	TotalCirculatingSupplyCollection = db.Collection(totalCirculatingSupplyCollName)
	CoinGeckoCollection = db.Collection(coinGeckoCollName)
	WalletCountCollections = db.Collection(walletCountCollName)
	DailyTransactionsVolumeCollection = db.Collection(dailyTransactionsVolumeCollName)
	EpochInfoCollection = db.Collection(epochInfoCollName)
	ValidatorHistoryCollection = db.Collection(validatorHistoryCollName)
	PriceHistoryCollection = db.Collection(priceHistoryCollName)
	TokenTransfersCollection = db.Collection(tokenTransfersCollName)
	TokenBalancesCollection = db.Collection(tokenBalancesCollName)
	PendingTransactionsCollection = db.Collection(pendingTransactionsCollName)
	GasHistoryCollection = db.Collection(gasHistoryCollName)
	SyncStateCollection = db.Collection(syncStateCollName)
	TokenMetadataCollection = db.Collection(tokenMetadataCollName)
	MarketTradesCollection = db.Collection(marketTradesCollName)
}

// marketTradesRetention bounds how long collected venue trades are kept.
// The longest chart the API serves is five days, so this leaves ample slack
// while keeping the collection from growing without limit. Changing it here
// does not retune an existing deployment: Mongo caches the TTL index's
// expiry, and lowering it later needs a collMod on the live index.
const marketTradesRetention = 45 * 24 * time.Hour

// ensureMarketTradesCollection creates the market trade collection and its
// indexes up front. Creating it eagerly (rather than letting the first
// insert do it) is what allows the index build to happen on an empty
// collection at startup instead of on a populated one under load.
func ensureMarketTradesCollection(db *mongo.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	exists, err := collectionExists(db, marketTradesCollName)
	if err != nil {
		log.Printf("Warning: could not check for %s collection: %v", marketTradesCollName, err)
		return
	}
	if !exists {
		if err := db.CreateCollection(ctx, marketTradesCollName); err != nil {
			// A concurrent creator winning the race is not an error worth
			// aborting on; the index build below still runs.
			log.Printf("Note: creating %s collection: %v", marketTradesCollName, err)
		}
	}

	indexes := []mongo.IndexModel{
		{
			// Serves both the windowed rollups and the coverage probe.
			Keys:    bson.D{{Key: "venue", Value: 1}, {Key: "at", Value: -1}},
			Options: options.Index().SetName("market_trades_venue_at_idx"),
		},
		{
			Keys: bson.D{{Key: "at", Value: 1}},
			Options: options.Index().
				SetName("market_trades_ttl_idx").
				SetExpireAfterSeconds(int32(marketTradesRetention.Seconds())),
		},
	}
	if _, err := db.Collection(marketTradesCollName).Indexes().CreateMany(ctx, indexes); err != nil {
		log.Printf("Warning: could not create indexes for %s: %v", marketTradesCollName, err)
		return
	}
	log.Printf("Market trade collection ready (retention %s)", marketTradesRetention)
}

func createIndexes(db *mongo.Database) {
	ctx := context.Background()

	// blocks collection indexes
	blocksIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "result.number", Value: -1},
				{Key: "result.timestamp", Value: 1},
			},
			Options: options.Index().SetName("result_number_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "result.hash", Value: 1},
			},
			Options: options.Index().SetName("result_hash"),
		},
		{
			// Backs the $elemMatch tx-by-hash lookup in ReturnSingleTransfer,
			// which scans blocks for a transaction whose hash matches the
			// query. Without this the lookup is a full collection scan.
			Keys: bson.D{
				{Key: "result.transactions.hash", Value: 1},
			},
			Options: options.Index().SetName("result_transactions_hash"),
		},
	}

	// transactionByAddress collection indexes
	transactionsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "timeStamp", Value: -1}},
			Options: options.Index().SetName("timestamp_desc"),
		},
		{
			Keys:    bson.D{{Key: "txHash", Value: 1}},
			Options: options.Index().SetName("tx_hash").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "timeStamp", Value: -1},
			},
			Options: options.Index().SetName("from_timestamp_desc"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "timeStamp", Value: -1},
			},
			Options: options.Index().SetName("to_timestamp_desc"),
		},
	}

	// addresses collection indexes
	addressesIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetName("id_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "balance", Value: -1}},
			Options: options.Index().SetName("balance_desc"),
		},
	}

	// internalTransactionByAddress collection indexes.
	// The compound (from|to, blockTimestamp desc) pair backs the $or+sort in
	// ReturnAllInternalTransactionsByAddress; the hash index backs the
	// per-tx lookups in GetInternalTransactionsByTxHash and
	// CountInternalTxsByTxHashes. The legacy single-field from/to indexes
	// are kept so existing query plans don't regress.
	internalTransactionsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "from", Value: 1}},
			Options: options.Index().SetName("internal_from"),
		},
		{
			Keys:    bson.D{{Key: "to", Value: 1}},
			Options: options.Index().SetName("internal_to"),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockTimestamp", Value: -1},
			},
			Options: options.Index().SetName("internal_from_blocktimestamp_desc"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockTimestamp", Value: -1},
			},
			Options: options.Index().SetName("internal_to_blocktimestamp_desc"),
		},
		{
			Keys:    bson.D{{Key: "hash", Value: 1}},
			Options: options.Index().SetName("internal_hash"),
		},
	}

	// contractCode collection indexes
	contractCodeIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "address", Value: 1}},
			Options: options.Index().SetName("contract_address_unique").SetUnique(true),
		},
		{
			// Backs GetContractByCreationTx (the /tx/:hash "contract created"
			// lookup). Sparse because most contractCode rows have no
			// creationTransaction field, so the index only holds the rows that
			// do, keeping it small.
			Keys:    bson.D{{Key: "creationTransaction", Value: 1}},
			Options: options.Index().SetName("contract_creation_tx").SetSparse(true),
		},
	}

	// transfer collection indexes
	transferIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "txHash", Value: 1}},
			Options: options.Index().SetName("transfer_txhash_unique").SetUnique(true),
		},
	}

	// tokenTransfers collection indexes.
	// The (contractAddress, blockNumberInt desc) compound backs the per-contract
	// transfer feed in GetTokenTransfers; the (from|to, blockNumberInt desc) pair
	// backs the by-holder $or+sort in GetTokenTransfersByAddress; the txHash
	// index backs the per-tx lookups in GetTokenTransfersByTxHash and
	// CountTokenTransfersByTxHashes.
	//
	// The compound sorts run on the numeric blockNumberInt field (true chain
	// order) rather than the hex string blockNumber (lexicographic). The index
	// names carry an _int suffix so they are created fresh on DBs that still
	// hold the older blockNumber-keyed indexes (the existence check is by name;
	// the legacy token_*_block_desc indexes become redundant and can be dropped
	// manually).
	tokenTransfersIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "blockNumberInt", Value: -1},
			},
			Options: options.Index().SetName("token_contract_block_int_desc"),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockNumberInt", Value: -1},
			},
			Options: options.Index().SetName("token_from_block_int_desc"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockNumberInt", Value: -1},
			},
			Options: options.Index().SetName("token_to_block_int_desc"),
		},
		{
			Keys:    bson.D{{Key: "txHash", Value: 1}},
			Options: options.Index().SetName("token_txhash"),
		},
	}

	// tokenBalances collection indexes.
	// holderAddress backs the per-wallet balance reads (GetTokenBalancesByAddress,
	// GetNFTBalancesByAddress) and the by-holder $group in GetTokenHolders.
	// contractAddress backs the per-contract reads (GetTokenHolders /
	// GetTokenInfo holder count). The (contractAddress, tokenID) compound backs
	// the tokenID-scoped holders path and the GetTokenIDs distinct-id aggregation.
	tokenBalancesIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "holderAddress", Value: 1}},
			Options: options.Index().SetName("token_balances_holder"),
		},
		{
			Keys:    bson.D{{Key: "contractAddress", Value: 1}},
			Options: options.Index().SetName("token_balances_contract"),
		},
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "tokenID", Value: 1},
			},
			Options: options.Index().SetName("token_balances_contract_tokenid"),
		},
	}

	// validators collection indexes (per-document model)
	validatorsIndexes := []mongo.IndexModel{
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
	}

	// Map of collection name -> indexes to create
	collections := map[string][]mongo.IndexModel{
		blocksCollName:                       blocksIndexes,
		transactionByAddressCollName:         transactionsIndexes,
		addressesCollName:                    addressesIndexes,
		internalTransactionByAddressCollName: internalTransactionsIndexes,
		contractCodeCollName:                 contractCodeIndexes,
		transferCollName:                     transferIndexes,
		validatorsCollName:                   validatorsIndexes,
		tokenTransfersCollName:               tokenTransfersIndexes,
		tokenBalancesCollName:                tokenBalancesIndexes,
	}

	for collName, indexes := range collections {
		// First check if collection exists
		exists, err := collectionExists(db, collName)
		if err != nil {
			log.Printf("Warning: Could not check if collection %s exists: %v", collName, err)
			continue
		}

		if !exists {
			log.Printf("Collection %s does not exist, skipping index creation", collName)
			continue
		}

		// Check if indexes already exist
		existingIndexes, err := getExistingIndexes(db, collName)
		if err != nil {
			log.Printf("Warning: Could not retrieve existing indexes for %s: %v", collName, err)
			continue
		}

		// Create only missing indexes
		var missingIndexes []mongo.IndexModel
		for _, idx := range indexes {
			if name := idx.Options.Name; name != nil {
				indexName := *name
				if !indexExists(existingIndexes, indexName) {
					missingIndexes = append(missingIndexes, idx)
				}
			} else {
				missingIndexes = append(missingIndexes, idx)
			}
		}

		if len(missingIndexes) == 0 {
			log.Printf("All required indexes for collection %s exist", collName)
			continue
		}

		// Create only missing indexes
		_, err = db.Collection(collName).Indexes().CreateMany(ctx, missingIndexes)
		if err != nil {
			log.Printf("Warning: Could not create indexes for %s: %v", collName, err)
		} else {
			log.Printf("Created missing indexes for collection %s", collName)
		}
	}
}

// collectionExists checks if a collection exists in the database
func collectionExists(db *mongo.Database, collectionName string) (bool, error) {
	collections, err := db.ListCollectionNames(context.Background(), bson.M{"name": collectionName})
	if err != nil {
		return false, err
	}
	return len(collections) > 0, nil
}

// getExistingIndexes retrieves all existing indexes for a collection
func getExistingIndexes(db *mongo.Database, collectionName string) ([]bson.M, error) {
	cursor, err := db.Collection(collectionName).Indexes().List(context.Background())
	if err != nil {
		return nil, err
	}

	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	return results, nil
}

// indexExists checks if an index with the given name exists in the collection
func indexExists(indexes []bson.M, indexName string) bool {
	for _, idx := range indexes {
		if name, ok := idx["name"].(string); ok && name == indexName {
			return true
		}
	}
	return false
}

// Initialize collections with fallback data
func initializeCollections(db *mongo.Database) {
	ctx := context.Background()

	// Initialize WalletCount collection with fallback data
	_, err := db.Collection(walletCountCollName).UpdateOne(
		ctx,
		bson.M{"_id": "current_count"},
		bson.M{"$setOnInsert": bson.M{"count": int64(0)}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize WalletCount collection: %v", err)
	}

	// Initialize dailyTransactionsVolume collection with fallback data
	_, err = db.Collection(dailyTransactionsVolumeCollName).UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{"volume": int64(0)}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize dailyTransactionsVolume collection: %v", err)
	}

	// Initialize totalCirculatingSupply collection with fallback data.
	// Keyed to the syncer's well-known _id so the seed and the syncer's
	// writer share one document; an unkeyed upsert used to insert a
	// random-_id doc the reader could pick up forever.
	_, err = db.Collection(totalCirculatingSupplyCollName).UpdateOne(
		ctx,
		bson.M{"_id": "totalBalance"},
		bson.M{"$setOnInsert": bson.M{"circulating": "0"}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize totalCirculatingSupply collection: %v", err)
	}

	// Initialize CoinGecko collection with fallback data
	_, err = db.Collection(coinGeckoCollName).UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$setOnInsert": bson.M{
			"marketCapUSD": 1000000000000000000.0,
			"priceUSD":     1000.0,
			"lastUpdated":  time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize CoinGecko collection: %v", err)
	}
}
