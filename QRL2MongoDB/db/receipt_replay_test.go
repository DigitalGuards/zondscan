package db

import (
	"QRL2MongoDB/models"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"testing"
)

func TestPendingReplayRetainsEnrichmentBehindExactIdentityFence(t *testing.T) {
	block := models.ZondDatabaseBlock{Result: models.Result{Number: "0x2", Hash: "0xabc", Transactions: []models.Transaction{{Data: "0x1234", Status: "0x0", GasUsed: "0x5208", EffectiveGasPrice: "0x7"}}}}
	err := refreshPendingBlockTransactions(block, func(_ context.Context, filter, update interface{}, _ ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
		fence, err := bson.MarshalExtJSON(filter, false, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"result.number", "0x2", "result.hash", "0xabc", "ingestionState", "$ne", "complete"} {
			if !strings.Contains(string(fence), want) {
				t.Fatalf("missing %s in fence %s", want, fence)
			}
		}
		payload, err := bson.MarshalExtJSON(update, false, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"result.transactions", `"data":"0x1234"`, `"gasUsed":"0x5208"`, `"effectiveGasPrice":"0x7"`, `"status":"0x0"`} {
			if !strings.Contains(string(payload), want) {
				t.Fatalf("missing %s in replay %s", want, payload)
			}
		}
		return &mongo.UpdateResult{MatchedCount: 1}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPendingReplayWriteFailureAndDisappearedIdentityStopCompletion(t *testing.T) {
	block := models.ZondDatabaseBlock{Result: models.Result{Number: "0x2", Hash: "0xabc"}}
	for _, mutationErr := range []error{nil, errors.New("write failed")} {
		err := refreshPendingBlockTransactions(block, func(context.Context, interface{}, interface{}, ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
			return &mongo.UpdateResult{}, mutationErr
		})
		if mutationErr != nil && !errors.Is(err, mutationErr) {
			t.Fatalf("lost write failure: %v", err)
		}
		if mutationErr == nil && !errors.Is(err, ErrBlockWriteUnresolved) {
			t.Fatalf("accepted missing identity: %v", err)
		}
	}
}
