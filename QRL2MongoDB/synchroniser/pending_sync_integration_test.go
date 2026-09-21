//go:build integration

package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPendingVerifierRevalidatesCanonicalReceiptAfterRollback(t *testing.T) {
	uri := os.Getenv("QRL_PENDING_INTEGRATION_URI")
	if uri == "" {
		t.Skip("QRL_PENDING_INTEGRATION_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	database := client.Database("qrl2-pending-verifier-integration")
	blocks := database.Collection(configs.BLOCKS_COLLECTION)
	pending := database.Collection(configs.PENDING_TRANSACTIONS_COLLECTION)
	for _, collection := range []*mongo.Collection{blocks, pending} {
		if _, err := collection.DeleteMany(ctx, bson.M{}); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, collection := range []*mongo.Collection{blocks, pending} {
			if _, err := collection.DeleteMany(cleanupCtx, bson.M{}); err != nil {
				t.Errorf("clean pending verifier integration collection: %v", err)
			}
		}
	}()

	previousBlocks := configs.BlocksCollections
	previousPending := configs.PendingTransactionsCollections
	configs.BlocksCollections = blocks
	configs.PendingTransactionsCollections = pending
	defer func() {
		configs.BlocksCollections = previousBlocks
		configs.PendingTransactionsCollections = previousPending
	}()

	hash := "0xtx"
	blockNumber := "0x2a"
	blockHash := "0xorphaned"
	if _, err := blocks.InsertOne(ctx, bson.M{
		"ingestionState": "complete",
		"result": bson.M{
			"number":       blockNumber,
			"hash":         blockHash,
			"transactions": bson.A{bson.M{"hash": hash}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pending.InsertOne(ctx, bson.M{"_id": hash, "status": "pending"}); err != nil {
		t.Fatal(err)
	}

	receipt := pendingVerificationTestReceipt(hash, blockNumber, blockHash)
	receiptFetched := make(chan struct{})
	waitingForMutationLock := make(chan struct{})
	type verificationResult struct {
		marked bool
		err    error
	}
	done := make(chan verificationResult, 1)

	chainMutationMu.Lock()
	go func() {
		_, marked, verifyErr := verifyPendingTransactionWithOperations(
			hash,
			func(string) (*models.TransactionReceipt, error) {
				close(receiptFetched)
				return receipt, nil
			},
			pendingVerificationOperations{
				withMutationLock: func(fn func() error) error {
					close(waitingForMutationLock)
					return WithChainMutationLock(fn)
				},
				countCanonicalBlocks: countCompletedCanonicalReceiptBlocks,
				markMined:            markPendingTransactionMined,
			},
		)
		done <- verificationResult{marked: marked, err: verifyErr}
	}()

	<-receiptFetched
	<-waitingForMutationLock
	if _, err := blocks.DeleteMany(ctx, bson.M{}); err != nil {
		chainMutationMu.Unlock()
		t.Fatal(err)
	}
	if _, err := pending.DeleteOne(ctx, bson.M{"_id": hash}); err != nil {
		chainMutationMu.Unlock()
		t.Fatal(err)
	}
	if _, err := pending.InsertOne(ctx, bson.M{
		"_id":        hash,
		"status":     "pending",
		"generation": "reinserted-after-rollback",
	}); err != nil {
		chainMutationMu.Unlock()
		t.Fatal(err)
	}
	chainMutationMu.Unlock()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.marked {
			t.Fatal("orphaned receipt marked the reinserted transaction mined")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pending verifier integration did not finish")
	}

	var row struct {
		Status     string `bson:"status"`
		Generation string `bson:"generation"`
	}
	if err := pending.FindOne(ctx, bson.M{"_id": hash}).Decode(&row); err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" || row.Generation != "reinserted-after-rollback" {
		t.Fatalf("reinserted pending row changed by stale receipt: %+v", row)
	}
}
