package db

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"backendAPI/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCanonicalCompanionPipelineFailsClosed(t *testing.T) {
	filter := bson.M{"hash": "0xabc"}
	got := canonicalCompanionPipeline(filter)
	want := []bson.M{
		{"$match": filter},
		{"$lookup": bson.M{
			"from": "blocks",
			"let":  bson.M{"companionBlockNumber": "$blockNumber"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$eq": bson.A{
					"$result.number", "$$companionBlockNumber",
				}}}},
				{"$limit": 2},
				{"$project": bson.M{"_id": 1, "ingestionState": 1}},
			},
			"as": canonicalBlockJoinAlias,
		}},
		{"$match": bson.M{"$expr": canonicalCompleteBlockJoinExpression()}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical companion pipeline = %#v, want %#v", got, want)
	}
}

func TestCanonicalTokenCompanionPipelineRequiresCompleteExactIdentity(t *testing.T) {
	filter := bson.M{"txHash": "0xabc"}
	got := canonicalTokenCompanionPipeline(filter)
	want := append(
		[]bson.M{{"$match": filter}},
		canonicalCompleteTokenBlockFenceStages("$blockNumber", "$blockHash")...,
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical token companion pipeline = %#v, want %#v", got, want)
	}

	expression := canonicalCompleteTokenBlockJoinExpression("$blockHash")
	wantExpression := bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{
			bson.M{"$size": "$" + canonicalTokenBlockJoinAlias},
			1,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".ingestionState", 0,
			}},
			completedBlockIngestionState,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".tokenIngestionState", 0,
			}},
			completedBlockIngestionState,
		}},
		bson.M{"$eq": bson.A{bson.M{"$type": "$blockHash"}, "string"}},
		bson.M{"$ne": bson.A{"$blockHash", ""}},
		bson.M{"$eq": bson.A{
			"$blockHash",
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".result.hash", 0,
			}},
		}},
	}}
	if !reflect.DeepEqual(expression, wantExpression) {
		t.Fatalf("token visibility expression = %#v, want %#v", expression, wantExpression)
	}
}

func TestCanonicalContractPipelineAllowsOnlyGenesisOrCompleteCreation(t *testing.T) {
	got := canonicalContractPipeline(bson.M{"address": "Qabc"})
	if len(got) != 3 {
		t.Fatalf("pipeline length = %d, want 3", len(got))
	}
	wantFinal := bson.M{"$match": bson.M{"$or": bson.A{
		bson.M{"genesisContract": true},
		bson.M{"$expr": canonicalCompleteBlockJoinExpression()},
	}}}
	if !reflect.DeepEqual(got[2], wantFinal) {
		t.Fatalf("contract visibility match = %#v, want %#v", got[2], wantFinal)
	}
}

func TestCanonicalVisibilityAgainstMongo(t *testing.T) {
	uri := os.Getenv("CANONICAL_VISIBILITY_MONGO_URI")
	if uri == "" {
		t.Skip("set CANONICAL_VISIBILITY_MONGO_URI to run the Mongo visibility test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping MongoDB: %v", err)
	}

	database := client.Database("zondscan_canonical_visibility_test")
	if err := database.Drop(ctx); err != nil {
		t.Fatalf("drop stale test database: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_ = database.Drop(dropCtx)
	})

	originals := struct {
		blocks, transactions, internals, transfers, tokenTransfers, contracts, tokenBalances, tokenMetadata *mongo.Collection
	}{
		configs.BlocksCollection,
		configs.TransactionByAddressCollection,
		configs.InternalTransactionByAddressCollection,
		configs.TransferCollections,
		configs.TokenTransfersCollection,
		configs.ContractInfoCollection,
		configs.TokenBalancesCollection,
		configs.TokenMetadataCollection,
	}
	configs.BlocksCollection = database.Collection("blocks")
	configs.TransactionByAddressCollection = database.Collection("transactionByAddress")
	configs.InternalTransactionByAddressCollection = database.Collection("internalTransactionByAddress")
	configs.TransferCollections = database.Collection("transfer")
	configs.TokenTransfersCollection = database.Collection("tokenTransfers")
	configs.ContractInfoCollection = database.Collection("contractCode")
	configs.TokenBalancesCollection = database.Collection("tokenBalances")
	configs.TokenMetadataCollection = database.Collection("tokenMetadata")
	t.Cleanup(func() {
		configs.BlocksCollection = originals.blocks
		configs.TransactionByAddressCollection = originals.transactions
		configs.InternalTransactionByAddressCollection = originals.internals
		configs.TransferCollections = originals.transfers
		configs.TokenTransfersCollection = originals.tokenTransfers
		configs.ContractInfoCollection = originals.contracts
		configs.TokenBalancesCollection = originals.tokenBalances
		configs.TokenMetadataCollection = originals.tokenMetadata
	})

	mustInsert := func(collection *mongo.Collection, documents ...interface{}) {
		t.Helper()
		if _, err := collection.InsertMany(ctx, documents); err != nil {
			t.Fatalf("insert %s fixtures: %v", collection.Name(), err)
		}
	}

	completeBlock := "0x1"
	pendingBlock := "0x2"
	missingBlock := "0x3"
	conflictBlock := "0x4"
	tokenPendingBlock := "0x5"
	wrongHashBlock := "0x6"
	missingHashBlock := "0x7"
	completeBlockHash := "0x" + strings.Repeat("a", 64)
	pendingBlockHash := "0x" + strings.Repeat("b", 64)
	conflictBlockHash := "0x" + strings.Repeat("c", 64)
	conflictOtherBlockHash := "0x" + strings.Repeat("d", 64)
	tokenPendingBlockHash := "0x" + strings.Repeat("e", 64)
	wrongHashCanonicalBlockHash := "0x" + strings.Repeat("f", 64)
	missingHashCanonicalBlockHash := "0x" + strings.Repeat("0", 64)
	mustInsert(configs.BlocksCollection,
		bson.M{
			"result":         bson.M{"number": completeBlock, "hash": completeBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "complete",
		},
		bson.M{
			"result":         bson.M{"number": pendingBlock, "hash": pendingBlockHash},
			"ingestionState": "pending", "tokenIngestionState": "pending",
		},
		bson.M{
			"result":         bson.M{"number": conflictBlock, "hash": conflictBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "complete",
		},
		bson.M{
			"result":         bson.M{"number": conflictBlock, "hash": conflictOtherBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "complete",
		},
		bson.M{
			"result":         bson.M{"number": tokenPendingBlock, "hash": tokenPendingBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "pending",
		},
		bson.M{
			"result":         bson.M{"number": wrongHashBlock, "hash": wrongHashCanonicalBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "complete",
		},
		bson.M{
			"result":         bson.M{"number": missingHashBlock, "hash": missingHashCanonicalBlockHash},
			"ingestionState": "complete", "tokenIngestionState": "complete",
		},
	)

	holder := "Q" + strings.Repeat("a", 128)
	other := "Q" + strings.Repeat("b", 128)
	contractComplete := "Q" + strings.Repeat("c", 128)
	contractPending := "Q" + strings.Repeat("d", 128)
	contractMissing := "Q" + strings.Repeat("e", 128)
	contractGenesis := "Q" + strings.Repeat("f", 128)
	contractConflict := "Q" + strings.Repeat("0", 128)
	hashComplete := "0x" + strings.Repeat("1", 64)
	hashPending := "0x" + strings.Repeat("2", 64)
	hashMissing := "0x" + strings.Repeat("3", 64)
	hashConflict := "0x" + strings.Repeat("4", 64)
	hashTokenPending := "0x" + strings.Repeat("5", 64)
	hashWrongBlockHash := "0x" + strings.Repeat("6", 64)
	hashHashless := "0x" + strings.Repeat("7", 64)

	for _, fixture := range []struct {
		block, hash, timestamp string
	}{
		{completeBlock, hashComplete, "0x1"},
		{pendingBlock, hashPending, "0x2"},
		{missingBlock, hashMissing, "0x3"},
		{conflictBlock, hashConflict, "0x4"},
	} {
		mustInsert(configs.TransactionByAddressCollection, bson.M{
			"from": holder, "to": other, "txHash": fixture.hash,
			"timeStamp": fixture.timestamp, "amount": 1.0, "blockNumber": fixture.block,
		})
		mustInsert(configs.InternalTransactionByAddressCollection, bson.M{
			"from": holder, "to": other, "hash": fixture.hash,
			"blockTimestamp": fixture.timestamp, "blockNumber": fixture.block,
		})
		mustInsert(configs.TransferCollections, bson.M{
			"from": holder, "to": other, "txHash": fixture.hash,
			"blockTimestamp": fixture.timestamp, "blockNumber": fixture.block,
		})
		mustInsert(configs.TokenBalancesCollection, bson.M{
			"contractAddress": contractComplete, "holderAddress": holder,
			"balance": "1", "blockNumber": fixture.block,
			"tokenID": "1", "tokenStandard": "ERC-721",
		})
	}

	mustInsert(configs.TokenTransfersCollection,
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashComplete, "blockNumber": completeBlock,
			"blockHash": completeBlockHash, "blockNumberInt": int64(1),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashPending, "blockNumber": pendingBlock,
			"blockHash": pendingBlockHash, "blockNumberInt": int64(2),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashMissing, "blockNumber": missingBlock,
			"blockHash": "0x" + strings.Repeat("8", 64), "blockNumberInt": int64(3),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashConflict, "blockNumber": conflictBlock,
			"blockHash": conflictBlockHash, "blockNumberInt": int64(4),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashTokenPending, "blockNumber": tokenPendingBlock,
			"blockHash": tokenPendingBlockHash, "blockNumberInt": int64(5),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashWrongBlockHash, "blockNumber": wrongHashBlock,
			"blockHash": "0x" + strings.Repeat("9", 64), "blockNumberInt": int64(6),
		},
		bson.M{
			"contractAddress": contractComplete, "from": holder, "to": other,
			"txHash": hashHashless, "blockNumber": missingHashBlock,
			"blockNumberInt": int64(7),
		},
	)

	mustInsert(configs.ContractInfoCollection,
		bson.M{
			"address": contractComplete, "contractCode": "0x01",
			"creationBlockNumber": completeBlock, "creationTransaction": hashComplete,
			"isToken": true, "tokenStandard": "ERC-721",
		},
		bson.M{
			"address": contractPending, "contractCode": "0x02",
			"creationBlockNumber": pendingBlock, "creationTransaction": hashPending,
		},
		bson.M{
			"address": contractMissing, "contractCode": "0x03",
			"creationBlockNumber": missingBlock, "creationTransaction": hashMissing,
		},
		bson.M{
			"address": contractGenesis, "contractCode": "0x04", "genesisContract": true,
		},
		bson.M{
			"address": contractConflict, "contractCode": "0x05",
			"creationBlockNumber": conflictBlock, "creationTransaction": hashConflict,
		},
	)

	transactions, err := ReturnTransactionsNetwork(1, 10)
	if err != nil || len(transactions) != 1 || transactions[0].TxHash != hashComplete {
		t.Fatalf("canonical network transactions = %#v, err %v", transactions, err)
	}
	transactionCount, err := CountTransactionsNetwork()
	if err != nil || transactionCount != 1 {
		t.Fatalf("canonical transaction count = %d, err %v", transactionCount, err)
	}
	latestTransactions, err := ReturnLatestTransactions()
	if err != nil || len(latestTransactions) != 1 || latestTransactions[0].TxHash != hashComplete {
		t.Fatalf("canonical latest transactions = %#v, err %v", latestTransactions, err)
	}
	addressTransactions, err := ReturnAllTransactionsByAddress(holder, 1, 10)
	if err != nil || len(addressTransactions) != 1 || addressTransactions[0].TxHash != hashComplete {
		t.Fatalf("canonical address transactions = %#v, err %v", addressTransactions, err)
	}
	addressCount, err := CountTransactions(holder)
	if err != nil || addressCount != 1 {
		t.Fatalf("canonical address transaction count = %d, err %v", addressCount, err)
	}
	nonZeroTransactions, err := ReturnNonZeroTransactions(holder, 1, 10)
	if err != nil || len(nonZeroTransactions) != 1 || nonZeroTransactions[0].TxHash != hashComplete {
		t.Fatalf("canonical nonzero transactions = %#v, err %v", nonZeroTransactions, err)
	}
	firstActivity, lastActivity, err := ReturnAddressActivityRange(holder)
	if err != nil || firstActivity != 1 || lastActivity != 1 {
		t.Fatalf("canonical activity range = (%d, %d), err %v", firstActivity, lastActivity, err)
	}
	internals, err := ReturnAllInternalTransactionsByAddress(holder, 1, 10)
	if err != nil || len(internals) != 1 || internals[0].Hash != hashComplete {
		t.Fatalf("canonical internal transactions = %#v, err %v", internals, err)
	}
	internalCount, err := CountInternalTransactionsByAddress(holder)
	if err != nil || internalCount != 1 {
		t.Fatalf("canonical internal count = %d, err %v", internalCount, err)
	}
	internalCounts, err := CountInternalTxsByTxHashes([]string{hashComplete, hashPending, hashMissing, hashConflict})
	if err != nil || len(internalCounts) != 1 || internalCounts[hashComplete] != 1 {
		t.Fatalf("canonical internal hash counts = %#v, err %v", internalCounts, err)
	}
	pendingInternals, err := GetInternalTransactionsByTxHash(hashPending)
	if err != nil || len(pendingInternals) != 0 {
		t.Fatalf("pending internal lookup = %#v, err %v", pendingInternals, err)
	}
	if pending, err := GetTransactionByHash(hashPending); err != nil || pending != nil {
		t.Fatalf("pending transfer lookup = %#v, err %v", pending, err)
	}
	if complete, err := GetTransactionByHash(hashComplete); err != nil || complete == nil {
		t.Fatalf("complete transfer lookup = %#v, err %v", complete, err)
	}

	tokenTransfers, tokenTransferCount, err := GetTokenTransfers(contractComplete, 0, 10)
	if err != nil || len(tokenTransfers) != 1 || tokenTransferCount != 1 ||
		tokenTransfers[0].TxHash != hashComplete || tokenTransfers[0].BlockHash != completeBlockHash {
		t.Fatalf("canonical token transfers = %#v, count %d, err %v", tokenTransfers, tokenTransferCount, err)
	}
	for label, txHash := range map[string]string{
		"core pending":       hashPending,
		"missing block":      hashMissing,
		"duplicate blocks":   hashConflict,
		"token pending":      hashTokenPending,
		"wrong block hash":   hashWrongBlockHash,
		"missing block hash": hashHashless,
	} {
		rejected, err := GetTokenTransfersByTxHash(txHash)
		if err != nil || len(rejected) != 0 {
			t.Fatalf("%s token transfers = %#v, err %v", label, rejected, err)
		}
	}
	addressTokenTransfers, addressTokenTransferCount, err := GetTokenTransfersByAddress(holder, 0, 10)
	if err != nil || len(addressTokenTransfers) != 1 || addressTokenTransferCount != 1 {
		t.Fatalf("canonical address token transfers = %#v, count %d, err %v", addressTokenTransfers, addressTokenTransferCount, err)
	}
	tokenCounts, err := CountTokenTransfersByTxHashes([]string{
		hashComplete,
		hashPending,
		hashMissing,
		hashConflict,
		hashTokenPending,
		hashWrongBlockHash,
		hashHashless,
	})
	if err != nil || len(tokenCounts) != 1 || tokenCounts[hashComplete] != 1 {
		t.Fatalf("canonical token hash counts = %#v, err %v", tokenCounts, err)
	}
	balances, err := GetTokenBalancesByAddress(holder, nil)
	if err != nil || len(balances) != 1 || balances[0].BlockNumber != completeBlock {
		t.Fatalf("canonical token balances = %#v, err %v", balances, err)
	}
	holders, holderCount, err := GetTokenHolders(contractComplete, "1", 0, 10)
	if err != nil || len(holders) != 1 || holderCount != 1 {
		t.Fatalf("canonical token holders = %#v, count %d, err %v", holders, holderCount, err)
	}
	tokenIDs, tokenIDCount, err := GetTokenIDs(contractComplete, 0, 10)
	if err != nil || len(tokenIDs) != 1 || tokenIDCount != 1 {
		t.Fatalf("canonical token IDs = %#v, count %d, err %v", tokenIDs, tokenIDCount, err)
	}
	nfts, err := GetNFTBalancesByAddress(holder, nil)
	if err != nil || len(nfts) != 1 || nfts[0].BlockNumber != completeBlock {
		t.Fatalf("canonical NFTs = %#v, err %v", nfts, err)
	}
	tokenInfo, err := GetTokenInfo(contractComplete)
	if err != nil || tokenInfo.HolderCount != 1 || tokenInfo.TransferCount != 1 {
		t.Fatalf("canonical token info = %#v, err %v", tokenInfo, err)
	}

	contracts, totalContracts, err := ReturnContracts(0, 10, "", nil, nil)
	if err != nil || len(contracts) != 2 || totalContracts != 2 {
		t.Fatalf("canonical contracts = %#v, count %d, err %v", contracts, totalContracts, err)
	}
	contractCount, err := CountContracts()
	if err != nil || contractCount != 2 {
		t.Fatalf("canonical contract count = %d, err %v", contractCount, err)
	}
	standardCounts, err := GetContractCountsByStandard()
	if err != nil || standardCounts.ERC721 != 1 || standardCounts.Other != 1 {
		t.Fatalf("canonical contract standard counts = %#v, err %v", standardCounts, err)
	}
	pendingContract, err := ReturnContractCode(contractPending)
	if err != nil || pendingContract.ContractAddress != "" {
		t.Fatalf("pending contract lookup = %#v, err %v", pendingContract, err)
	}
	completeContracts, err := GetContractsByAddresses([]string{contractComplete, contractPending, contractMissing, contractConflict})
	if err != nil || len(completeContracts) != 1 {
		t.Fatalf("canonical contracts by address = %#v, err %v", completeContracts, err)
	}
	if pending, err := GetContractByCreationTx(hashPending); err != nil || pending != nil {
		t.Fatalf("pending creation lookup = %#v, err %v", pending, err)
	}
	if complete, err := GetContractByCreationTx(hashComplete); err != nil || complete == nil {
		t.Fatalf("complete creation lookup = %#v, err %v", complete, err)
	}
}
