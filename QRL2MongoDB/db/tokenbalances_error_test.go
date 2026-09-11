package db

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type failingTokenBalanceMutationStore struct {
	deleteManyErr  error
	deleteOneErr   error
	updateOneErr   error
	deleteManyCall int
	deleteOneCall  int
	updateOneCall  int
}

func (store *failingTokenBalanceMutationStore) DeleteMany(
	context.Context,
	interface{},
	...*options.DeleteOptions,
) (*mongo.DeleteResult, error) {
	store.deleteManyCall++
	return &mongo.DeleteResult{}, store.deleteManyErr
}

func (store *failingTokenBalanceMutationStore) DeleteOne(
	context.Context,
	interface{},
	...*options.DeleteOptions,
) (*mongo.DeleteResult, error) {
	store.deleteOneCall++
	return &mongo.DeleteResult{}, store.deleteOneErr
}

func (store *failingTokenBalanceMutationStore) UpdateOne(
	context.Context,
	interface{},
	interface{},
	...*options.UpdateOptions,
) (*mongo.UpdateResult, error) {
	store.updateOneCall++
	return &mongo.UpdateResult{}, store.updateOneErr
}

func installTokenBalanceMutationTestDependencies(
	t *testing.T,
	store tokenBalanceMutationStore,
	owner string,
	balance *big.Int,
) {
	t.Helper()
	previousStore := getTokenBalanceMutationStore
	previousOwner := getERC721OwnerForBalanceStore
	previousBalance := getERC1155ForBalanceStore
	getTokenBalanceMutationStore = func() tokenBalanceMutationStore { return store }
	getERC721OwnerForBalanceStore = func(string, *big.Int) (string, error) { return owner, nil }
	getERC1155ForBalanceStore = func(string, string, *big.Int) (*big.Int, error) { return balance, nil }
	t.Cleanup(func() {
		getTokenBalanceMutationStore = previousStore
		getERC721OwnerForBalanceStore = previousOwner
		getERC1155ForBalanceStore = previousBalance
	})
}

func TestStoreERC721OwnershipPropagatesBurnDeleteFailure(t *testing.T) {
	deleteErr := errors.New("burn delete failed")
	store := &failingTokenBalanceMutationStore{deleteManyErr: deleteErr}
	installTokenBalanceMutationTestDependencies(t, store, "", big.NewInt(0))

	err := StoreERC721Ownership("Q"+strings.Repeat("1", 128), big.NewInt(7), "0x10")
	if !errors.Is(err, deleteErr) {
		t.Fatalf("burn delete error = %v, want wrapped sentinel", err)
	}
	if store.deleteManyCall != 1 || store.updateOneCall != 0 {
		t.Fatalf("mutation calls after burn failure: delete=%d update=%d",
			store.deleteManyCall, store.updateOneCall)
	}
}

func TestStoreERC721OwnershipPropagatesPriorOwnerDeleteFailure(t *testing.T) {
	deleteErr := errors.New("prior owner delete failed")
	store := &failingTokenBalanceMutationStore{deleteManyErr: deleteErr}
	owner := "Q" + strings.Repeat("2", 128)
	installTokenBalanceMutationTestDependencies(t, store, owner, big.NewInt(0))

	err := StoreERC721Ownership("Q"+strings.Repeat("1", 128), big.NewInt(7), "0x10")
	if !errors.Is(err, deleteErr) {
		t.Fatalf("prior owner delete error = %v, want wrapped sentinel", err)
	}
	if store.deleteManyCall != 1 || store.updateOneCall != 0 {
		t.Fatalf("mutation calls after prior-owner failure: delete=%d update=%d",
			store.deleteManyCall, store.updateOneCall)
	}
}

func TestStoreERC1155BalancePropagatesZeroBalanceDeleteFailure(t *testing.T) {
	deleteErr := errors.New("zero balance delete failed")
	store := &failingTokenBalanceMutationStore{deleteOneErr: deleteErr}
	installTokenBalanceMutationTestDependencies(t, store, "", big.NewInt(0))

	err := StoreERC1155Balance(
		"Q"+strings.Repeat("1", 128),
		"Q"+strings.Repeat("2", 128),
		big.NewInt(9),
		"0x10",
	)
	if !errors.Is(err, deleteErr) {
		t.Fatalf("zero balance delete error = %v, want wrapped sentinel", err)
	}
	if store.deleteOneCall != 1 || store.updateOneCall != 0 {
		t.Fatalf("mutation calls after zero-balance failure: delete=%d update=%d",
			store.deleteOneCall, store.updateOneCall)
	}
}

func TestStoreERC1155BalancePropagatesNoDocumentDeleteError(t *testing.T) {
	store := &failingTokenBalanceMutationStore{deleteOneErr: mongo.ErrNoDocuments}
	installTokenBalanceMutationTestDependencies(t, store, "", big.NewInt(0))

	err := StoreERC1155Balance(
		"Q"+strings.Repeat("1", 128),
		"Q"+strings.Repeat("2", 128),
		big.NewInt(9),
		"0x10",
	)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("no-document delete error = %v, want wrapped sentinel", err)
	}
}
