package db

import (
	"context"
	"reflect"
	"testing"
	"time"

	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
)

func TestContractExplainChallengeConsumeFilterBindsIdentityAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC)
	challenge := models.ContractExplainChallenge{
		ID:              "nonce",
		Version:         "version",
		Action:          "action",
		Origin:          "https://zondscan.com",
		ChainID:         "0x539",
		Method:          "POST",
		Route:           "/contract/explain/Qcontract?regenerate=1",
		ContractAddress: "Qcontract",
		SignerAddress:   "Qsigner",
		Message:         []byte("message"),
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(time.Minute),
	}
	want := bson.D{
		{Key: "_id", Value: challenge.ID},
		{Key: "version", Value: challenge.Version},
		{Key: "action", Value: challenge.Action},
		{Key: "origin", Value: challenge.Origin},
		{Key: "chainId", Value: challenge.ChainID},
		{Key: "method", Value: challenge.Method},
		{Key: "route", Value: challenge.Route},
		{Key: "contractAddress", Value: challenge.ContractAddress},
		{Key: "signerAddress", Value: challenge.SignerAddress},
		{Key: "message", Value: challenge.Message},
		{Key: "issuedAt", Value: challenge.IssuedAt},
		{Key: "expiresAt", Value: bson.M{"$eq": challenge.ExpiresAt, "$gt": now}},
	}
	if got := contractExplainChallengeConsumeFilter(challenge, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestContractExplainChallengeStoreFailsClosedWithoutCollection(t *testing.T) {
	store := NewContractExplainChallengeStore(nil)
	if err := store.Create(context.Background(), models.ContractExplainChallenge{}); err == nil {
		t.Fatal("Create succeeded without a collection")
	}
	if _, _, err := store.Load(context.Background(), "nonce"); err == nil {
		t.Fatal("Load succeeded without a collection")
	}
	if _, err := store.Consume(context.Background(), models.ContractExplainChallenge{}, time.Now()); err == nil {
		t.Fatal("Consume succeeded without a collection")
	}
}
