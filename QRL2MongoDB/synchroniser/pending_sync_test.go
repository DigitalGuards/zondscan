package synchroniser

import (
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func pendingVerificationTestReceipt(hash, blockNumber, blockHash string) *models.TransactionReceipt {
	receipt := &models.TransactionReceipt{}
	receipt.Result.TransactionHash = hash
	receipt.Result.BlockNumber = blockNumber
	receipt.Result.BlockHash = blockHash
	receipt.Result.Status = "0x1"
	return receipt
}

func immediatePendingMutationLock(fn func() error) error {
	return fn()
}

func TestCompletedCanonicalReceiptFilterFencesExactBlockAndTransaction(t *testing.T) {
	hash := "0xtx"
	receipt := pendingVerificationTestReceipt(hash, "0x2a", "0xblock")

	filter, err := completedCanonicalReceiptFilter(hash, receipt)
	if err != nil {
		t.Fatal(err)
	}
	want := bson.M{
		"ingestionState":           db.BlockIngestionComplete,
		"result.number":            "0x2a",
		"result.hash":              "0xblock",
		"result.transactions.hash": hash,
	}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("canonical receipt filter = %#v, want %#v", filter, want)
	}
}

func TestCompletedCanonicalReceiptFilterRejectsIncompleteOrMismatchedIdentity(t *testing.T) {
	hash := "0xtx"
	tests := []struct {
		name    string
		receipt *models.TransactionReceipt
	}{
		{name: "nil receipt"},
		{name: "missing block number", receipt: pendingVerificationTestReceipt(hash, "", "0xblock")},
		{name: "missing block hash", receipt: pendingVerificationTestReceipt(hash, "0x2a", "")},
		{name: "different transaction", receipt: pendingVerificationTestReceipt("0xother", "0x2a", "0xblock")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := completedCanonicalReceiptFilter(hash, test.receipt); err == nil {
				t.Fatal("expected receipt identity validation failure")
			}
		})
	}
}

func TestPendingReceiptRequiresExactlyOneCompleteCanonicalBlock(t *testing.T) {
	hash := "0xtx"
	receipt := pendingVerificationTestReceipt(hash, "0x2a", "0xblock")
	tests := []struct {
		name          string
		canonical     int64
		wantMarked    bool
		wantMarkCalls int
		wantErr       bool
	}{
		{name: "orphaned receipt", canonical: 0},
		{name: "one canonical block", canonical: 1, wantMarked: true, wantMarkCalls: 1},
		{name: "duplicate canonical blocks", canonical: 2, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			markCalls := 0
			_, marked, err := verifyPendingTransactionWithOperations(
				hash,
				func(string) (*models.TransactionReceipt, error) { return receipt, nil },
				pendingVerificationOperations{
					withMutationLock: immediatePendingMutationLock,
					countCanonicalBlocks: func(bson.M) (int64, error) {
						return test.canonical, nil
					},
					markMined: func(string) (bool, error) {
						markCalls++
						return true, nil
					},
				},
			)
			if marked != test.wantMarked {
				t.Fatalf("marked mined = %v, want %v", marked, test.wantMarked)
			}
			if markCalls != test.wantMarkCalls {
				t.Fatalf("mark calls = %d, want %d", markCalls, test.wantMarkCalls)
			}
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestPendingReceiptFetchedBeforeRollbackCannotTombstoneReinsertedTransaction(t *testing.T) {
	hash := "0xtx"
	receipt := pendingVerificationTestReceipt(hash, "0x2a", "0xorphaned")
	var mutationMu sync.Mutex
	mutationMu.Lock()
	receiptFetched := make(chan struct{})
	waitingForMutationLock := make(chan struct{})
	type verificationResult struct {
		marked bool
		err    error
	}
	done := make(chan verificationResult, 1)

	canonicalCount := int64(1)
	rollbackFinished := false
	markCalls := 0
	go func() {
		_, marked, err := verifyPendingTransactionWithOperations(
			hash,
			func(string) (*models.TransactionReceipt, error) {
				close(receiptFetched)
				return receipt, nil
			},
			pendingVerificationOperations{
				withMutationLock: func(fn func() error) error {
					close(waitingForMutationLock)
					mutationMu.Lock()
					defer mutationMu.Unlock()
					return fn()
				},
				countCanonicalBlocks: func(bson.M) (int64, error) {
					if !rollbackFinished {
						return 0, errors.New("canonical check ran before rollback completed")
					}
					return canonicalCount, nil
				},
				markMined: func(string) (bool, error) {
					markCalls++
					return true, nil
				},
			},
		)
		done <- verificationResult{marked: marked, err: err}
	}()

	<-receiptFetched
	<-waitingForMutationLock
	// Simulate rollback deleting the old complete block and making the tx
	// eligible for a fresh pending insert while the verifier waits at the same
	// canonical mutation boundary.
	canonicalCount = 0
	rollbackFinished = true
	mutationMu.Unlock()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.marked {
			t.Fatal("orphaned receipt marked the reinserted pending transaction mined")
		}
		if markCalls != 0 {
			t.Fatalf("mark calls = %d, want 0", markCalls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending receipt verification did not finish")
	}
}
