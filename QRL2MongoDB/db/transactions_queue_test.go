package db

import (
	"QRL2MongoDB/models"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func queueMutationJSON(t *testing.T, value interface{}) string {
	t.Helper()
	encoded, err := bson.MarshalExtJSON(bson.M{"value": value}, false, false)
	if err != nil {
		t.Fatalf("marshal queue mutation: %v", err)
	}
	return string(encoded)
}

func TestPendingTokenClaimUsesServerTimeAndAttemptToken(t *testing.T) {
	filterJSON := queueMutationJSON(t, pendingTokenClaimFilter())
	if strings.Count(filterJSON, "$$NOW") != 2 {
		t.Fatalf("claim filter server-time references = %d, want 2: %s",
			strings.Count(filterJSON, "$$NOW"), filterJSON)
	}
	if !strings.Contains(filterJSON, "processingUntil") ||
		!strings.Contains(filterJSON, "nextAttemptAt") {
		t.Fatalf("claim filter lacks retry or expiry fence: %s", filterJSON)
	}

	updateJSON := queueMutationJSON(t, pendingTokenClaimUpdate("claim-123"))
	for _, required := range []string{
		"$$NOW",
		"$dateAdd",
		"claim-123",
		"processingToken",
		"attempts",
	} {
		if !strings.Contains(updateJSON, required) {
			t.Fatalf("claim update lacks %q: %s", required, updateJSON)
		}
	}
}

func TestPendingTokenClaimCompletionIsOwnershipPinned(t *testing.T) {
	id := primitive.NewObjectID()
	filter := pendingTokenClaimOwnerFilter(pendingTokenContractWork{
		ID:              id,
		ContractAddress: "Qcontract",
		TxHash:          "0xtx",
		ProcessingToken: "claim-123",
	})
	if filter["processingToken"] != "claim-123" {
		t.Fatalf("owner filter token = %#v", filter["processingToken"])
	}
	if filter["processing"] != true {
		t.Fatalf("owner filter processing = %#v", filter["processing"])
	}
	if filter["_id"] != id {
		t.Fatalf("owner filter _id = %#v, want %s", filter["_id"], id.Hex())
	}
	if _, ok := filter["contractAddress"]; ok {
		t.Fatalf("owner filter falls back to mutable address key: %#v", filter)
	}
}

func TestPendingTokenRetryBackoffIsBounded(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 0, want: 5 * time.Second},
		{attempts: 1, want: 5 * time.Second},
		{attempts: 2, want: 10 * time.Second},
		{attempts: 8, want: 10 * time.Minute},
		{attempts: 1000, want: 10 * time.Minute},
	}
	for _, test := range tests {
		if got := pendingTokenRetryDelay(test.attempts); got != test.want {
			t.Fatalf("retry delay for attempt %d = %s, want %s",
				test.attempts, got, test.want)
		}
	}
}

func TestPendingTokenFailureUsesServerTimeAndReleasesClaim(t *testing.T) {
	updateJSON := queueMutationJSON(t, pendingTokenFailureUpdate("rpc unavailable", 30*time.Second))
	for _, required := range []string{
		"$$NOW",
		"$dateAdd",
		"nextAttemptAt",
		"processingToken",
		"rpc unavailable",
	} {
		if !strings.Contains(updateJSON, required) {
			t.Fatalf("failure update lacks %q: %s", required, updateJSON)
		}
	}
}

func TestPendingTokenClaimRenewalUsesServerTime(t *testing.T) {
	updateJSON := queueMutationJSON(t, pendingTokenRenewClaimUpdate())
	for _, required := range []string{"$$NOW", "$dateAdd", "processingUntil"} {
		if !strings.Contains(updateJSON, required) {
			t.Fatalf("renewal update lacks %q: %s", required, updateJSON)
		}
	}
}

func TestPendingTokenClaimSortMatchesQueueIndexOrder(t *testing.T) {
	sort := pendingTokenClaimSort()
	want := []string{"nextAttemptAt", "processingUntil", "createdAt", "_id"}
	if len(sort) != len(want) {
		t.Fatalf("sort length = %d, want %d", len(sort), len(want))
	}
	for index, field := range want {
		if sort[index].Key != field || sort[index].Value != 1 {
			t.Fatalf("sort[%d] = %#v, want %s ascending", index, sort[index], field)
		}
	}
}

func TestPendingTokenQueueStopSignal(t *testing.T) {
	stopCh := make(chan struct{})
	if pendingTokenQueueStopped([]<-chan struct{}{stopCh}) {
		t.Fatal("open stop channel reported stopped")
	}
	close(stopCh)
	if !pendingTokenQueueStopped([]<-chan struct{}{stopCh}) {
		t.Fatal("closed stop channel was ignored")
	}
	if pendingTokenQueueStopped(nil) {
		t.Fatal("missing stop channel reported stopped")
	}
}

func TestNewPendingTokenContractWorkCanonicalizesIdentity(t *testing.T) {
	addressBody := strings.Repeat("A", 128)
	txHashBody := strings.Repeat("B", 64)
	blockHashBody := strings.Repeat("C", 64)
	work, err := newPendingTokenContractWork(
		"0X"+addressBody,
		&models.Transaction{
			Hash:        "0X" + txHashBody,
			BlockNumber: "0X000A",
			BlockHash:   "0X" + blockHashBody,
		},
		"0X000F",
	)
	if err != nil {
		t.Fatal(err)
	}
	if work.ContractAddress != "Q"+strings.ToLower(addressBody) {
		t.Fatalf("contract address = %s", work.ContractAddress)
	}
	if work.TxHash != "0x"+strings.ToLower(txHashBody) ||
		work.BlockHash != "0x"+strings.ToLower(blockHashBody) ||
		work.BlockNumber != "0xa" || work.BlockTimestamp != "0xf" {
		t.Fatalf("canonical work = %+v", work)
	}
}

func TestNewPendingTokenContractWorkRejectsMissingIdentity(t *testing.T) {
	validAddress := "Q" + strings.Repeat("1", 128)
	validTxHash := "0x" + strings.Repeat("2", 64)
	tests := []struct {
		name      string
		address   string
		tx        *models.Transaction
		timestamp string
	}{
		{name: "nil transaction", address: validAddress},
		{name: "invalid contract", address: "Qbad", tx: &models.Transaction{}},
		{
			name:    "missing block hash",
			address: validAddress,
			tx: &models.Transaction{
				Hash:        validTxHash,
				BlockNumber: "0x1",
			},
			timestamp: "0x2",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newPendingTokenContractWork(test.address, test.tx, test.timestamp); err == nil {
				t.Fatal("invalid queue identity accepted")
			}
		})
	}
}

func pendingTokenIdentityFixture() (pendingTokenContractWork, *models.TransactionReceipt) {
	contract := "Q" + strings.Repeat("1", 128)
	txHash := "0x" + strings.Repeat("2", 64)
	blockHash := "0x" + strings.Repeat("3", 64)
	work := pendingTokenContractWork{
		ContractAddress: contract,
		TxHash:          txHash,
		BlockNumber:     "0xa",
		BlockHash:       blockHash,
		BlockTimestamp:  "0xf",
	}
	receipt := &models.TransactionReceipt{}
	receipt.Result.TransactionHash = txHash
	receipt.Result.BlockNumber = "0x0A"
	receipt.Result.BlockHash = blockHash
	receipt.Result.Status = "0x1"
	receipt.Result.To = "0x" + strings.Repeat("1", 128)
	return work, receipt
}

func TestValidatePendingTokenReceiptIdentity(t *testing.T) {
	work, receipt := pendingTokenIdentityFixture()
	if err := validatePendingTokenReceiptIdentity(work, receipt); err != nil {
		t.Fatal(err)
	}

	receipt.Result.TransactionHash = "0x" + strings.Repeat("4", 64)
	if err := validatePendingTokenReceiptIdentity(work, receipt); err == nil ||
		!strings.Contains(err.Error(), "does not match queued hash") {
		t.Fatalf("mismatched receipt error = %v", err)
	}
	if err := validatePendingTokenReceiptIdentity(work, nil); err == nil {
		t.Fatal("nil receipt accepted")
	}
}

func TestValidatePendingTokenReceiptRejectsUnboundLog(t *testing.T) {
	work, receipt := pendingTokenIdentityFixture()
	receipt.Result.Logs = []models.Log{{
		Address:         work.ContractAddress,
		TransactionHash: work.TxHash,
		BlockNumber:     work.BlockNumber,
		BlockHash:       "0x" + strings.Repeat("4", 64),
		LogIndex:        "0x0",
	}}
	if err := validatePendingTokenReceiptIdentity(work, receipt); err == nil ||
		!strings.Contains(err.Error(), "does not match receipt block hash") {
		t.Fatalf("unbound log error = %v", err)
	}
}

func TestPendingTokenCanonicalBlockFilterRequiresCompleteIdentity(t *testing.T) {
	work, receipt := pendingTokenIdentityFixture()
	filterJSON := queueMutationJSON(t, pendingTokenCanonicalBlockFilter(work, receipt))
	for _, required := range []string{
		BlockIngestionComplete,
		"blockNumberInt",
		"result.number",
		"result.hash",
		"result.transactions",
		"$elemMatch",
		"blocknumber",
		"blockhash",
		work.TxHash,
		work.BlockHash,
	} {
		if !strings.Contains(filterJSON, required) {
			t.Fatalf("canonical filter lacks %q: %s", required, filterJSON)
		}
	}
}
