package synchroniser

import (
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func tokenQueueBlock() models.ZondDatabaseBlock {
	blockHash := "0x" + strings.Repeat("a", 64)
	block := models.ZondDatabaseBlock{}
	block.Result.Number = "0x2a"
	block.Result.Hash = blockHash
	block.Result.Timestamp = "0x100"
	block.Result.TransactionsRoot = "0x" + strings.Repeat("c", 64)
	block.Result.Transactions = []models.Transaction{{
		Hash:             "0x" + strings.Repeat("b", 64),
		BlockNumber:      "0x2a",
		BlockHash:        blockHash,
		TransactionIndex: "0x0",
	}}
	return block
}

func TestTokenBlockClaimUsesServerTimeAndExactPendingState(t *testing.T) {
	filter, err := tokenBlockClaimFilter("0x2a")
	if err != nil {
		t.Fatal(err)
	}
	if filter["ingestionState"] != db.BlockIngestionComplete ||
		filter["tokenIngestionState"] != TokenIngestionPending ||
		filter["blockNumberInt"] != int64(42) {
		t.Fatalf("claim filter = %#v", filter)
	}
	filterJSON, err := bson.MarshalExtJSON(filter, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(filterJSON), "$$NOW") ||
		!strings.Contains(string(filterJSON), "tokenProcessingUntil") ||
		!strings.Contains(string(filterJSON), "tokenNextAttemptAt") {
		t.Fatalf("claim filter lacks server-time fences: %s", filterJSON)
	}

	updateJSON, err := bson.MarshalExtJSON(
		bson.M{"pipeline": tokenBlockClaimUpdate("owner-token")},
		false,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"$$NOW", "$dateAdd", "owner-token", "tokenAttempts"} {
		if !strings.Contains(string(updateJSON), required) {
			t.Fatalf("claim update lacks %q: %s", required, updateJSON)
		}
	}
	work := tokenBlockWork{
		ID:                   "block-row",
		Result:               tokenQueueBlock().Result,
		TokenProcessingToken: "owner-token",
	}
	ownerJSON, err := bson.MarshalExtJSON(tokenBlockOwnerFilter(work), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ownerJSON), "$$NOW") ||
		!strings.Contains(string(ownerJSON), "tokenProcessingUntil") ||
		!strings.Contains(string(ownerJSON), "$gt") {
		t.Fatalf("owner filter lacks lease freshness fence: %s", ownerJSON)
	}
	renewalJSON, err := bson.MarshalExtJSON(
		bson.M{"pipeline": tokenBlockRenewalUpdate()},
		false,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(renewalJSON), "$$NOW") ||
		!strings.Contains(string(renewalJSON), "$dateAdd") {
		t.Fatalf("renewal update lacks server time: %s", renewalJSON)
	}
}

func TestTokenBlockRetryDelayIsBounded(t *testing.T) {
	if got := tokenBlockRetryDelay(1); got != 5*time.Second {
		t.Fatalf("first retry delay = %s", got)
	}
	if got := tokenBlockRetryDelay(1000); got != 10*time.Minute {
		t.Fatalf("bounded retry delay = %s", got)
	}
	failureJSON, err := bson.MarshalExtJSON(
		bson.M{"pipeline": tokenBlockFailureUpdate(strings.Repeat("x", 4096), time.Minute)},
		false,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(failureJSON), "lastTokenError") ||
		!strings.Contains(string(failureJSON), "lastTokenFailedAt") ||
		!strings.Contains(string(failureJSON), "tokenNextAttemptAt") {
		t.Fatalf("failure update = %s", failureJSON)
	}
}

func TestTokenBlockQueueIndexesCoverClaimsAndOldestOrdering(t *testing.T) {
	models := tokenBlockQueueIndexModels()
	if len(models) != 2 {
		t.Fatalf("token block queue index count = %d, want 2", len(models))
	}
	tests := []struct {
		position int
		name     string
		keys     []string
	}{
		{
			position: 0,
			name:     "token_ingestion_claim_idx",
			keys: []string{
				"ingestionState",
				"tokenIngestionState",
				"tokenNextAttemptAt",
				"tokenProcessingUntil",
				"blockNumberInt",
				"_id",
			},
		},
		{
			position: 1,
			name:     "token_ingestion_oldest_idx",
			keys: []string{
				"ingestionState",
				"tokenIngestionState",
				"blockNumberInt",
				"_id",
			},
		},
	}
	for _, test := range tests {
		model := models[test.position]
		if model.Options == nil || model.Options.Name == nil || *model.Options.Name != test.name {
			t.Fatalf("index %d name = %#v, want %s", test.position, model.Options, test.name)
		}
		keys, ok := model.Keys.(bson.D)
		if !ok {
			t.Fatalf("index %s keys type = %T", test.name, model.Keys)
		}
		if len(keys) != len(test.keys) {
			t.Fatalf("index %s key count = %d, want %d", test.name, len(keys), len(test.keys))
		}
		for position, key := range test.keys {
			if keys[position].Key != key || keys[position].Value != 1 {
				t.Fatalf("index %s key %d = %#v, want %s ascending",
					test.name, position, keys[position], key)
			}
		}
	}
}

func TestIncompleteTokenBlockFilterCoversLegacyAndPendingStates(t *testing.T) {
	encoded, err := bson.MarshalExtJSON(incompleteTokenBlockFilter(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	filter := string(encoded)
	for _, required := range []string{
		db.BlockIngestionComplete,
		TokenIngestionComplete,
		"tokenIngestionState",
		"$ne",
	} {
		if !strings.Contains(filter, required) {
			t.Fatalf("incomplete token block filter lacks %q: %s", required, filter)
		}
	}
	if strings.Contains(filter, "result.transactions") {
		t.Fatalf("incomplete token block filter excludes empty invalid states: %s", filter)
	}
}

func TestInvalidTokenBlockStateFilterAllowsOnlyPendingOrComplete(t *testing.T) {
	encoded, err := bson.MarshalExtJSON(invalidTokenBlockStateFilter(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	filter := string(encoded)
	for _, required := range []string{
		db.BlockIngestionComplete,
		TokenIngestionPending,
		TokenIngestionComplete,
		"$nin",
	} {
		if !strings.Contains(filter, required) {
			t.Fatalf("invalid token state filter lacks %q: %s", required, filter)
		}
	}
}

func TestTokenBlockSnapshotChangesWithTransactionMembership(t *testing.T) {
	block := tokenQueueBlock()
	first, err := tokenBlockSnapshot(block)
	if err != nil {
		t.Fatal(err)
	}
	changed := block
	changed.Result.Transactions = append([]models.Transaction(nil), block.Result.Transactions...)
	changed.Result.Transactions[0].Hash = "0x" + strings.Repeat("d", 64)
	second, err := tokenBlockSnapshot(changed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, ",") == strings.Join(second, ",") {
		t.Fatal("transaction membership change did not change the block snapshot")
	}
}

func TestPendingTokenBlockWorkerRetriesOnQuietHead(t *testing.T) {
	stopCh := make(chan struct{})
	var calls atomic.Int32
	called := make(chan struct{}, 4)
	process := func() error {
		count := calls.Add(1)
		select {
		case called <- struct{}{}:
		default:
		}
		if count == 1 {
			return errors.New("retryable block log failure")
		}
		return nil
	}
	if !startPendingTokenBlockWorker(stopCh, 5*time.Millisecond, process) {
		t.Fatal("token block worker was not registered")
	}
	for pass := 0; pass < 2; pass++ {
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatalf("token block worker pass %d did not run", pass+1)
		}
	}
	close(stopCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := WaitForBackgroundWorkers(ctx); err != nil {
		t.Fatal(err)
	}
}
