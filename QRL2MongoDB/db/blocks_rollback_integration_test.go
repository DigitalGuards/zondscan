//go:build integration

package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/rpc"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func rollbackIntegrationAddress(fill string) string {
	return "Q" + strings.Repeat(fill, 128)
}

func rollbackIntegrationCount(t *testing.T, collection *mongo.Collection, filter interface{}) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func TestRollbackInvalidatesSnapshotsAndRestoresPendingEligibility(t *testing.T) {
	uri := os.Getenv("QRL_ROLLBACK_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_ROLLBACK_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	var hello struct {
		SetName string `bson:"setName"`
		Msg     string `bson:"msg"`
	}
	if err := client.Database("admin").RunCommand(
		ctx,
		bson.D{{Key: "hello", Value: 1}},
	).Decode(&hello); err != nil {
		t.Fatal(err)
	}
	if hello.SetName == "" && hello.Msg != "isdbgrid" {
		t.Fatal("integration MongoDB does not support transactions")
	}

	database := client.Database("qrldata-z")
	testCollections := []string{
		configs.ADDRESSES_COLLECTION,
		configs.BLOCKS_COLLECTION,
		configs.INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION,
		configs.TRANSACTION_BY_ADDRESS_COLLECTION,
		configs.TRANSFER_COLLECTION,
		configs.PENDING_TRANSACTIONS_COLLECTION,
		configs.CONTRACT_CODE_COLLECTION,
		configs.CONTRACT_VERIFICATIONS_COLLECTION,
		configs.BALANCE_RECONCILIATIONS_COLLECTION,
		configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION,
		"tokenTransfers",
		"tokenBalances",
		"pending_token_contracts",
		SyncStateCollection,
	}
	clearTestCollections := func(cleanupCtx context.Context) error {
		for _, name := range testCollections {
			if _, err := database.Collection(name).DeleteMany(cleanupCtx, bson.M{}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := clearTestCollections(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := clearTestCollections(cleanupCtx); err != nil {
			t.Errorf("clean rollback integration collections: %v", err)
		}
	}()

	previousDB := configs.DB
	previousAddresses := configs.AddressesCollections
	previousBlocks := configs.BlocksCollections
	previousInternal := configs.InternalTransactionByAddressCollections
	previousTransactions := configs.TransactionByAddressCollections
	previousTransfers := configs.TransferCollections
	previousPending := configs.PendingTransactionsCollections
	previousDeadLetters := configs.TokenEventDeadLettersCollection
	configs.DB = client
	configs.AddressesCollections = database.Collection(configs.ADDRESSES_COLLECTION)
	configs.BlocksCollections = database.Collection(configs.BLOCKS_COLLECTION)
	configs.InternalTransactionByAddressCollections = database.Collection(
		configs.INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION,
	)
	configs.TransactionByAddressCollections = database.Collection(
		configs.TRANSACTION_BY_ADDRESS_COLLECTION,
	)
	configs.TransferCollections = database.Collection(configs.TRANSFER_COLLECTION)
	configs.PendingTransactionsCollections = database.Collection(configs.PENDING_TRANSACTIONS_COLLECTION)
	configs.TokenEventDeadLettersCollection = database.Collection(configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION)
	defer func() {
		configs.DB = previousDB
		configs.AddressesCollections = previousAddresses
		configs.BlocksCollections = previousBlocks
		configs.InternalTransactionByAddressCollections = previousInternal
		configs.TransactionByAddressCollections = previousTransactions
		configs.TransferCollections = previousTransfers
		configs.PendingTransactionsCollections = previousPending
		configs.TokenEventDeadLettersCollection = previousDeadLetters
	}()

	addressA := rollbackIntegrationAddress("1")
	addressB := rollbackIntegrationAddress("2")
	addressC := rollbackIntegrationAddress("3")
	addressD := rollbackIntegrationAddress("4")
	createdFromTransfer := rollbackIntegrationAddress("5")
	createdFromContractDoc := rollbackIntegrationAddress("6")
	minerAddress := rollbackIntegrationAddress("7")
	withdrawalAddress := rollbackIntegrationAddress("8")
	contract20 := rollbackIntegrationAddress("a")
	contract721 := rollbackIntegrationAddress("b")
	contract1155 := rollbackIntegrationAddress("c")
	minedHash := "0xmined"
	pendingHash := "0xpending"
	blockNumber := "0x11"
	blockHash := "0xblock11"

	blocks := database.Collection(configs.BLOCKS_COLLECTION)
	if _, err := blocks.InsertOne(ctx, bson.M{
		"blockNumberInt": int64(17),
		"ingestionState": BlockIngestionComplete,
		"result": bson.M{
			"number": blockNumber,
			"hash":   blockHash,
			"miner":  minerAddress,
			"withdrawals": bson.A{
				bson.M{"address": withdrawalAddress},
				bson.M{"address": configs.QRLZeroAddress},
				bson.M{"address": "Qinvalid"},
			},
			"transactions": bson.A{
				bson.M{"hash": minedHash, "from": addressA, "to": addressB},
				bson.M{"hash": pendingHash, "from": addressB, "to": ""},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	addresses := database.Collection(configs.ADDRESSES_COLLECTION)
	addressDocs := make([]interface{}, 0, 8)
	for _, address := range []string{
		addressA,
		addressB,
		addressC,
		addressD,
		createdFromTransfer,
		createdFromContractDoc,
		minerAddress,
		withdrawalAddress,
	} {
		addressDocs = append(addressDocs, bson.M{
			"id":         address,
			"balance":    1.0,
			"isContract": address == createdFromTransfer || address == createdFromContractDoc,
		})
	}
	if _, err := addresses.InsertMany(ctx, addressDocs); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Collection(configs.TRANSFER_COLLECTION).InsertOne(ctx, bson.M{
		"blockNumber":     blockNumber,
		"txHash":          minedHash,
		"from":            addressA,
		"contractAddress": createdFromTransfer,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Collection(configs.TRANSACTION_BY_ADDRESS_COLLECTION).InsertOne(ctx, bson.M{
		"blockNumber": blockNumber,
		"txHash":      minedHash,
		"from":        addressA,
		"to":          addressB,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Collection(configs.INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION).InsertOne(ctx, bson.M{
		"blockNumber": blockNumber,
		"hash":        minedHash,
		"type":        "CALL",
		"from":        addressC,
		"to":          addressD,
	}); err != nil {
		t.Fatal(err)
	}

	contracts := database.Collection(configs.CONTRACT_CODE_COLLECTION)
	if _, err := contracts.InsertMany(ctx, []interface{}{
		bson.M{
			"address":             createdFromTransfer,
			"creationBlockNumber": blockNumber,
			"creationBlockHash":   blockHash,
			"creationTransaction": minedHash,
		},
		bson.M{
			"address":             createdFromContractDoc,
			"creationBlockNumber": blockNumber,
			"creationBlockHash":   blockHash,
			"creationTransaction": pendingHash,
		},
	}); err != nil {
		t.Fatal(err)
	}

	tokenTransfers := database.Collection("tokenTransfers")
	if _, err := tokenTransfers.InsertMany(ctx, []interface{}{
		bson.M{
			"contractAddress": contract20,
			"from":            addressA,
			"to":              addressB,
			"blockNumber":     blockNumber,
			"txHash":          minedHash,
		},
		bson.M{
			"contractAddress": contract721,
			"from":            addressA,
			"to":              addressB,
			"blockNumber":     blockNumber,
			"txHash":          minedHash,
			"tokenStandard":   rpc.StandardERC721,
			"tokenID":         "7",
		},
		bson.M{
			"contractAddress": contract1155,
			"from":            configs.QRLZeroAddress,
			"to":              addressC,
			"blockNumber":     blockNumber,
			"txHash":          minedHash,
			"tokenStandard":   rpc.StandardERC1155,
			"tokenID":         "9",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Collection(configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION).InsertOne(ctx, bson.M{
		"blockNumber":    blockNumber,
		"blockNumberInt": int64(17),
		"blockHash":      blockHash,
		"txHash":         minedHash,
		"logIndex":       "0x0",
		"emitter":        contract20,
		"topic0":         rpc.TransferEventSignature,
		"tokenStandard":  rpc.StandardERC20,
		"reason":         "deterministic malformed payload",
		"firstSeenAt":    time.Now().UTC(),
		"lastSeenAt":     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	tokenBalances := database.Collection("tokenBalances")
	if _, err := tokenBalances.InsertMany(ctx, []interface{}{
		bson.M{
			"contractAddress": contract20,
			"holderAddress":   addressA,
			"balance":         "1",
			"blockNumber":     blockNumber,
			"updatedAt":       "2026-08-28T00:00:00Z",
		},
		bson.M{
			"contractAddress": contract20,
			"holderAddress":   addressB,
			"balance":         "2",
			"blockNumber":     blockNumber,
			"updatedAt":       "2026-08-28T00:00:00Z",
		},
		bson.M{
			"contractAddress": contract721,
			"holderAddress":   addressB,
			"tokenStandard":   rpc.StandardERC721,
			"tokenID":         "7",
			"balance":         "1",
			"blockNumber":     blockNumber,
			"updatedAt":       "2026-08-28T00:00:00Z",
		},
		bson.M{
			"contractAddress": contract1155,
			"holderAddress":   addressC,
			"tokenStandard":   rpc.StandardERC1155,
			"tokenID":         "9",
			"balance":         "3",
			"blockNumber":     blockNumber,
			"updatedAt":       "2026-08-28T00:00:00Z",
		},
	}); err != nil {
		t.Fatal(err)
	}

	pending := database.Collection(configs.PENDING_TRANSACTIONS_COLLECTION)
	if _, err := pending.InsertMany(ctx, []interface{}{
		bson.M{"_id": minedHash, "status": "mined"},
		bson.M{"_id": pendingHash, "status": "pending"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Collection("pending_token_contracts").InsertOne(ctx, bson.M{
		"contractAddress": contract20,
		"txHash":          minedHash,
		"blockNumber":     blockNumber,
		"processed":       false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Collection(SyncStateCollection).InsertOne(ctx, bson.M{
		"_id":              LastSyncedBlockID,
		"block_number":     blockNumber,
		"block_number_int": int64(17),
	}); err != nil {
		t.Fatal(err)
	}
	latest := GetLatestBlockFromDB()
	if latest == nil || latest.Result.Number != blockNumber || latest.Result.Hash != blockHash {
		t.Fatalf("latest complete block identity = %+v", latest)
	}

	if err := Rollback("0x10"); err != nil {
		t.Fatal(err)
	}

	for name, collection := range map[string]*mongo.Collection{
		"blocks":                       blocks,
		"transfer":                     database.Collection(configs.TRANSFER_COLLECTION),
		"transactionByAddress":         database.Collection(configs.TRANSACTION_BY_ADDRESS_COLLECTION),
		"internalTransactionByAddress": database.Collection(configs.INTERNAL_TRANSACTION_BY_ADDRESS_COLLECTION),
		"tokenTransfers":               tokenTransfers,
		"tokenEventDeadLetters":        database.Collection(configs.TOKEN_EVENT_DEAD_LETTERS_COLLECTION),
		"pending_token_contracts":      database.Collection("pending_token_contracts"),
		"contractCode":                 contracts,
	} {
		if count := rollbackIntegrationCount(t, collection, bson.M{}); count != 0 {
			t.Fatalf("%s retained %d orphaned rows", name, count)
		}
	}
	if count := rollbackIntegrationCount(t, pending, bson.M{"_id": minedHash}); count != 0 {
		t.Fatalf("mined tombstone count = %d, want 0", count)
	}
	if count := rollbackIntegrationCount(t, pending, bson.M{"_id": pendingHash}); count != 1 {
		t.Fatalf("pending row count = %d, want 1", count)
	}
	if count := rollbackIntegrationCount(t, addresses, bson.M{"balanceStale": true}); count != 8 {
		t.Fatalf("stale native balance count = %d, want 8", count)
	}
	if count := rollbackIntegrationCount(t, addresses, bson.M{
		"id":         bson.M{"$in": []string{createdFromTransfer, createdFromContractDoc}},
		"isContract": true,
	}); count != 0 {
		t.Fatalf("orphaned contract flag count = %d, want 0", count)
	}
	if count := rollbackIntegrationCount(t, tokenBalances, bson.M{"balanceStale": true}); count != 4 {
		t.Fatalf("stale token balance count = %d, want 4", count)
	}
	if count := rollbackIntegrationCount(
		t,
		database.Collection(configs.BALANCE_RECONCILIATIONS_COLLECTION),
		bson.M{},
	); count != 12 {
		t.Fatalf("balance reconciliation count = %d, want 12", count)
	}

	var syncState struct {
		BlockNumber    string `bson:"block_number"`
		BlockNumberInt int64  `bson:"block_number_int"`
	}
	if err := database.Collection(SyncStateCollection).FindOne(
		ctx,
		bson.M{"_id": LastSyncedBlockID},
	).Decode(&syncState); err != nil {
		t.Fatal(err)
	}
	if syncState.BlockNumber != "0x10" || syncState.BlockNumberInt != 16 {
		t.Fatalf("sync state after rollback = %+v", syncState)
	}
}
