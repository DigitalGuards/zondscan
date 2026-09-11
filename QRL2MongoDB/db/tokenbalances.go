package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type tokenBalanceMutationStore interface {
	DeleteMany(context.Context, interface{}, ...*options.DeleteOptions) (*mongo.DeleteResult, error)
	DeleteOne(context.Context, interface{}, ...*options.DeleteOptions) (*mongo.DeleteResult, error)
	UpdateOne(context.Context, interface{}, interface{}, ...*options.UpdateOptions) (*mongo.UpdateResult, error)
}

var (
	getERC721OwnerForBalanceStore = rpc.GetERC721Owner
	getERC1155ForBalanceStore     = rpc.GetERC1155Balance
	getTokenBalanceMutationStore  = func() tokenBalanceMutationStore {
		collection := configs.GetTokenBalancesCollection()
		if collection == nil {
			return nil
		}
		return collection
	}
)

func canonicalTokenBalanceObservation(
	blockNumber string,
	blockHash string,
) (string, int64, string, error) {
	canonicalNumber, err := canonicalHexQuantity(blockNumber, "token balance block number")
	if err != nil {
		return "", 0, "", err
	}
	number := new(big.Int)
	number.SetString(strings.TrimPrefix(canonicalNumber, "0x"), 16)
	if !number.IsInt64() {
		return "", 0, "", fmt.Errorf("token balance block number exceeds int64: %s", canonicalNumber)
	}
	if blockHash != "" {
		blockHash, err = canonicalFixedHash(blockHash, "token balance block hash")
		if err != nil {
			return "", 0, "", err
		}
	}
	return canonicalNumber, number.Int64(), blockHash, nil
}

// hasNewerTokenBalanceObservation rejects only strictly newer observations.
// An equal-height row may carry an orphaned block hash and must remain
// replaceable by the exact canonical replay. The synchroniser serializes the
// caller with rollback and enforces one unique canonical block identity, so a
// stale equal-height writer cannot arrive after the replacement.
func hasNewerTokenBalanceObservation(filter bson.M, blockNumberInt int64) (bool, error) {
	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		return false, fmt.Errorf("token balances collection is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var existing struct {
		BlockNumber    string `bson:"blockNumber"`
		BlockNumberInt *int64 `bson:"blockNumberInt"`
	}
	// Rollback deliberately retains snapshot rows and marks them stale before
	// exact canonical replay. Exclude them from the ordering fence, even when
	// their orphaned height is numerically above the replayed observation.
	observationFilter := bson.M{"$and": bson.A{
		filter,
		bson.M{"balanceStale": bson.M{"$ne": true}},
	}}
	err := collection.FindOne(
		ctx,
		observationFilter,
		options.FindOne().
			SetProjection(bson.M{
				"blockNumber":    1,
				"blockNumberInt": 1,
			}).
			SetSort(bson.D{{Key: "blockNumberInt", Value: -1}}),
	).Decode(&existing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load token balance observation: %w", err)
	}
	existingInt := int64(0)
	if existing.BlockNumberInt != nil {
		existingInt = *existing.BlockNumberInt
	} else {
		_, parsed, _, err := canonicalTokenBalanceObservation(existing.BlockNumber, "")
		if err != nil {
			return false, fmt.Errorf("decode legacy token balance observation: %w", err)
		}
		existingInt = parsed
	}
	return existingInt > blockNumberInt, nil
}

// StoreTokenBalance updates the token balance for a given address
func StoreTokenBalance(contractAddress string, holderAddress string, amount string, blockNumber string) error {
	return storeTokenBalanceWithGetter(
		contractAddress,
		holderAddress,
		amount,
		blockNumber,
		"",
		false,
		rpc.GetTokenBalance,
	)
}

// StoreTokenBalanceAtBlock stores the ERC-20 balance observed at blockHash.
func StoreTokenBalanceAtBlock(
	contractAddress string,
	holderAddress string,
	amount string,
	blockNumber string,
	blockHash string,
) error {
	return storeTokenBalanceWithGetter(
		contractAddress,
		holderAddress,
		amount,
		blockNumber,
		blockHash,
		true,
		func(address string, holder string) (string, error) {
			return rpc.GetTokenBalanceAtBlock(address, holder, blockHash)
		},
	)
}

func storeTokenBalanceWithGetter(
	contractAddress string,
	holderAddress string,
	amount string,
	blockNumber string,
	blockHash string,
	enforceOrdering bool,
	getBalance func(string, string) (string, error),
) error {
	// Normalize addresses to canonical Q-prefix form
	contractAddress = validation.ConvertToQAddress(contractAddress)

	// Debug-level log for per-record operations; Info is reserved for batch summaries.
	configs.Logger.Debug("Attempting to store token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.String("transferAmount", amount),
		zap.String("blockNumber", blockNumber))

	// Normalize holder address to canonical Q-prefix form
	holderAddress = validation.ConvertToQAddress(holderAddress)

	// Special handling for zero address (QRL uses Q prefix)
	if validation.IsZeroAddress(holderAddress) {
		configs.Logger.Debug("Skipping token balance update for zero address",
			zap.String("holderAddress", holderAddress))
		return nil
	}

	var err error
	blockNumber, blockNumberInt, blockHash, err := canonicalTokenBalanceObservation(
		blockNumber,
		blockHash,
	)
	if err != nil {
		return err
	}
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
		"$and": bson.A{
			bson.M{"$or": bson.A{
				bson.M{"tokenStandard": rpc.StandardERC20},
				bson.M{"tokenStandard": bson.M{"$exists": false}},
			}},
			bson.M{"$or": bson.A{
				bson.M{"tokenID": ""},
				bson.M{"tokenID": bson.M{"$exists": false}},
			}},
		},
	}
	if enforceOrdering {
		stale, err := hasNewerTokenBalanceObservation(filter, blockNumberInt)
		if err != nil {
			return err
		}
		if stale {
			configs.Logger.Debug("Skipping stale ERC-20 balance observation",
				zap.String("contractAddress", contractAddress),
				zap.String("holderAddress", holderAddress),
				zap.String("blockNumber", blockNumber),
				zap.String("blockHash", blockHash))
			return nil
		}
	}

	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		configs.Logger.Error("Failed to get token balances collection")
		return fmt.Errorf("token balances collection is nil")
	}

	// Get current balance from RPC with more robust error handling
	configs.Logger.Debug("Calling RPC to get current token balance")
	balance, err := getBalance(contractAddress, holderAddress)
	if err != nil {
		configs.Logger.Error("Failed to get token balance from RPC",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		return fmt.Errorf("get token balance: %w", err)
	} else {
		configs.Logger.Debug("Retrieved current token balance",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.String("balance", balance))
	}

	// Create update document. Phase 2 tags every ERC-20 row with
	// tokenStandard="ERC-20" so backend / frontend queries can filter by
	// standard without having to JOIN against contractCode. Legacy rows
	// without the field are still uniquely keyed by (contract, holder)
	// because Mongo treats their missing tokenID as null.
	set := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
		"balance":         balance,
		"blockNumber":     blockNumber,
		"blockNumberInt":  blockNumberInt,
		"updatedAt":       time.Now().UTC().Format(time.RFC3339),
		"tokenStandard":   rpc.StandardERC20,
	}
	unset := bson.M{
		"balanceStale":   "",
		"balanceStaleAt": "",
	}
	if blockHash != "" {
		set["blockHash"] = blockHash
	} else {
		unset["blockHash"] = ""
	}
	update := bson.M{"$set": set, "$unset": unset}

	// Update options
	opts := options.Update().SetUpsert(true)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform upsert
	result, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update token balance in database",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		return fmt.Errorf("failed to update token balance: %w", err)
	}

	configs.Logger.Debug("Token balance update completed",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.Int64("matchedCount", result.MatchedCount),
		zap.Int64("modifiedCount", result.ModifiedCount),
		zap.Int64("upsertedCount", result.UpsertedCount))

	return nil
}

// StoreERC721Ownership refreshes the single-owner row for one ERC-721
// (contract, tokenID) pair via `ownerOf(uint256)` and reconciles the
// (contract, holder, tokenID) rows accordingly.
//
// Behaviour:
//   - Transport error from the RPC layer leaves existing rows untouched
//     (C5 promote-only invariant: a transient blip must not delete state).
//   - Contract revert / empty return => no current owner; delete any stale
//     row that claimed this id. Sparse storage; burned NFTs leave no row.
//   - Owner returned => upsert (contract, owner, tokenID) with balance="1"
//     and tokenStandard="ERC-721". Single-owner invariant: any other row
//     with the same (contract, tokenID) but a different holder is deleted.
//
// TODO(scale): One RPC call per ERC-721 transfer event. Testnet OK; mainnet
// at high NFT volume would prefer an event-delta accumulator with periodic
// reconciliation. See GetERC721Owner for the same note.
func StoreERC721Ownership(contractAddress string, tokenID *big.Int, blockNumber string) error {
	return storeERC721OwnershipWithGetter(
		contractAddress,
		tokenID,
		blockNumber,
		"",
		false,
		getERC721OwnerForBalanceStore,
	)
}

// StoreERC721OwnershipAtBlock refreshes ownership from the exact state
// selected by blockHash.
func StoreERC721OwnershipAtBlock(
	contractAddress string,
	tokenID *big.Int,
	blockNumber string,
	blockHash string,
) error {
	return storeERC721OwnershipWithGetter(
		contractAddress,
		tokenID,
		blockNumber,
		blockHash,
		true,
		func(address string, id *big.Int) (string, error) {
			return rpc.GetERC721OwnerAtBlock(address, id, blockHash)
		},
	)
}

func storeERC721OwnershipWithGetter(
	contractAddress string,
	tokenID *big.Int,
	blockNumber string,
	blockHash string,
	enforceOrdering bool,
	getOwner func(string, *big.Int) (string, error),
) error {
	if tokenID == nil {
		return fmt.Errorf("tokenID required")
	}
	contractAddress = validation.ConvertToQAddress(contractAddress)
	blockNumber, blockNumberInt, blockHash, err := canonicalTokenBalanceObservation(
		blockNumber,
		blockHash,
	)
	if err != nil {
		return err
	}
	idStr := tokenID.String()
	observationFilter := bson.M{
		"contractAddress": contractAddress,
		"tokenID":         idStr,
		"tokenStandard":   rpc.StandardERC721,
	}
	if enforceOrdering {
		stale, err := hasNewerTokenBalanceObservation(observationFilter, blockNumberInt)
		if err != nil {
			return err
		}
		if stale {
			configs.Logger.Debug("Skipping stale ERC-721 ownership observation",
				zap.String("contract", contractAddress),
				zap.String("tokenID", idStr),
				zap.String("blockNumber", blockNumber),
				zap.String("blockHash", blockHash))
			return nil
		}
	}

	configs.Logger.Debug("Refreshing ERC-721 ownership",
		zap.String("contract", contractAddress),
		zap.String("tokenID", tokenID.String()),
		zap.String("blockNumber", blockNumber))

	owner, err := getOwner(contractAddress, tokenID)
	if err != nil {
		// Transport / unmarshal failure. Preserve existing state.
		configs.Logger.Warn("GetERC721Owner transport error; preserving existing balance state",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID.String()),
			zap.Error(err))
		return err
	}

	collection := getTokenBalanceMutationStore()
	if collection == nil {
		return fmt.Errorf("token balances collection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// No current owner (burned / never minted): drop any stale row.
	if owner == "" {
		_, dErr := collection.DeleteMany(ctx, bson.M{
			"contractAddress": contractAddress,
			"tokenID":         idStr,
			"tokenStandard":   rpc.StandardERC721,
		})
		if dErr != nil {
			configs.Logger.Error("Failed to delete stale ERC-721 ownership row",
				zap.String("contract", contractAddress),
				zap.String("tokenID", idStr),
				zap.Error(dErr))
			return fmt.Errorf("delete burned ERC-721 ownership row: %w", dErr)
		}
		return nil
	}

	owner = validation.ConvertToQAddress(owner)

	// Delete any stale row for this (contract, tokenID) held by anyone other
	// than the current owner. Enforces the single-owner invariant.
	_, dErr := collection.DeleteMany(ctx, bson.M{
		"contractAddress": contractAddress,
		"tokenID":         idStr,
		"tokenStandard":   rpc.StandardERC721,
		"holderAddress":   bson.M{"$ne": owner},
	})
	if dErr != nil {
		configs.Logger.Error("Failed to delete stale ERC-721 ownership row(s) for prior holder",
			zap.String("contract", contractAddress),
			zap.String("tokenID", idStr),
			zap.Error(dErr))
		return fmt.Errorf("delete prior ERC-721 ownership row: %w", dErr)
	}

	// Upsert the current owner's row.
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   owner,
		"tokenID":         idStr,
	}
	set := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   owner,
		"tokenID":         idStr,
		"tokenStandard":   rpc.StandardERC721,
		"balance":         "1",
		"blockNumber":     blockNumber,
		"blockNumberInt":  blockNumberInt,
		"updatedAt":       time.Now().UTC().Format(time.RFC3339),
	}
	unset := bson.M{
		"balanceStale":   "",
		"balanceStaleAt": "",
	}
	if blockHash != "" {
		set["blockHash"] = blockHash
	} else {
		unset["blockHash"] = ""
	}
	update := bson.M{"$set": set, "$unset": unset}
	if _, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		configs.Logger.Error("Failed to upsert ERC-721 ownership row",
			zap.String("contract", contractAddress),
			zap.String("holder", owner),
			zap.String("tokenID", idStr),
			zap.Error(err))
		return err
	}

	return nil
}

// StoreERC1155Balance refreshes a holder's per-id balance for one ERC-1155
// (contract, holder, tokenID) tuple via `balanceOf(address,uint256)`.
//
// Behaviour:
//   - Transport error: preserve existing state (C5 invariant), propagate err.
//   - Balance == 0 (incl. contract revert): delete the row (sparse storage).
//   - Balance > 0: upsert (contract, holder, tokenID) with the decimal-string
//     quantity and tokenStandard="ERC-1155".
//
// TODO(scale): One RPC call per (from, to) side per ERC-1155 transfer. See
// GetERC1155Balance for the accumulator/reconciliation note.
func StoreERC1155Balance(contractAddress, holderAddress string, tokenID *big.Int, blockNumber string) error {
	return storeERC1155BalanceWithGetter(
		contractAddress,
		holderAddress,
		tokenID,
		blockNumber,
		"",
		false,
		getERC1155ForBalanceStore,
	)
}

// StoreERC1155BalanceAtBlock refreshes one holder balance from the exact
// state selected by blockHash.
func StoreERC1155BalanceAtBlock(
	contractAddress string,
	holderAddress string,
	tokenID *big.Int,
	blockNumber string,
	blockHash string,
) error {
	return storeERC1155BalanceWithGetter(
		contractAddress,
		holderAddress,
		tokenID,
		blockNumber,
		blockHash,
		true,
		func(address string, holder string, id *big.Int) (*big.Int, error) {
			return rpc.GetERC1155BalanceAtBlock(address, holder, id, blockHash)
		},
	)
}

func storeERC1155BalanceWithGetter(
	contractAddress string,
	holderAddress string,
	tokenID *big.Int,
	blockNumber string,
	blockHash string,
	enforceOrdering bool,
	getBalance func(string, string, *big.Int) (*big.Int, error),
) error {
	if tokenID == nil {
		return fmt.Errorf("tokenID required")
	}
	contractAddress = validation.ConvertToQAddress(contractAddress)
	holderAddress = validation.ConvertToQAddress(holderAddress)

	// Skip zero-address; mint/burn endpoints don't hold balances.
	if validation.IsZeroAddress(holderAddress) {
		return nil
	}
	blockNumber, blockNumberInt, blockHash, err := canonicalTokenBalanceObservation(
		blockNumber,
		blockHash,
	)
	if err != nil {
		return err
	}
	idStr := tokenID.String()
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
		"tokenID":         idStr,
		"tokenStandard":   rpc.StandardERC1155,
	}
	if enforceOrdering {
		stale, err := hasNewerTokenBalanceObservation(filter, blockNumberInt)
		if err != nil {
			return err
		}
		if stale {
			configs.Logger.Debug("Skipping stale ERC-1155 balance observation",
				zap.String("contract", contractAddress),
				zap.String("holder", holderAddress),
				zap.String("tokenID", idStr),
				zap.String("blockNumber", blockNumber),
				zap.String("blockHash", blockHash))
			return nil
		}
	}

	configs.Logger.Debug("Refreshing ERC-1155 balance",
		zap.String("contract", contractAddress),
		zap.String("holder", holderAddress),
		zap.String("tokenID", tokenID.String()),
		zap.String("blockNumber", blockNumber))

	balance, err := getBalance(contractAddress, holderAddress, tokenID)
	if err != nil {
		configs.Logger.Warn("GetERC1155Balance transport error; preserving existing balance state",
			zap.String("contract", contractAddress),
			zap.String("holder", holderAddress),
			zap.String("tokenID", tokenID.String()),
			zap.Error(err))
		return err
	}

	collection := getTokenBalanceMutationStore()
	if collection == nil {
		return fmt.Errorf("token balances collection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sparse storage: delete the row when balance is zero.
	if balance == nil || balance.Sign() == 0 {
		if _, dErr := collection.DeleteOne(ctx, filter); dErr != nil {
			configs.Logger.Error("Failed to delete zero ERC-1155 balance row",
				zap.String("contract", contractAddress),
				zap.String("holder", holderAddress),
				zap.String("tokenID", idStr),
				zap.Error(dErr))
			return fmt.Errorf("delete zero ERC-1155 balance row: %w", dErr)
		}
		return nil
	}

	set := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
		"tokenID":         idStr,
		"tokenStandard":   rpc.StandardERC1155,
		"balance":         balance.String(),
		"blockNumber":     blockNumber,
		"blockNumberInt":  blockNumberInt,
		"updatedAt":       time.Now().UTC().Format(time.RFC3339),
	}
	unset := bson.M{
		"balanceStale":   "",
		"balanceStaleAt": "",
	}
	if blockHash != "" {
		set["blockHash"] = blockHash
	} else {
		unset["blockHash"] = ""
	}
	update := bson.M{"$set": set, "$unset": unset}
	if _, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		configs.Logger.Error("Failed to upsert ERC-1155 balance row",
			zap.String("contract", contractAddress),
			zap.String("holder", holderAddress),
			zap.String("tokenID", idStr),
			zap.Error(err))
		return err
	}

	return nil
}

// InitializeTokenBalancesCollection sets up tokenBalances indexes, including
// the Phase 2 per-tokenID migration.
//
// Index plan:
//
//   - UNIQUE (contractAddress, holderAddress, tokenID): primary key. Legacy
//     ERC-20 rows lack `tokenID`, Mongo treats missing fields as `null` for
//     index purposes, so the legacy (contract, holder, null) rows remain
//     unique by extension of the prior (contract, holder) unique. No
//     migration collision.
//   - secondary (contractAddress, tokenID, holderAddress): drives per-id
//     holder lookups (`/token/<addr>/holders?tokenID=`).
//   - secondary (holderAddress) and (contractAddress): unchanged, support
//     address-page and token-page sweeps.
//
// Phase 2 migration:
//
//   - Create the new 3-tuple unique BEFORE dropping the legacy 2-tuple unique.
//     A partial failure (e.g. transient duplicate-key on legacy data) leaves
//     the old unique in place, never an unguarded collection.
//   - Drop the legacy index by name only after the new one is online. Errors
//     other than IndexNotFound are warnings; the new unique is the source of
//     truth either way.
//
// Note on the old `address_idx` / `contract_address_idx` indexes from #88:
// those targeted a non-existent `address` field (storage writes
// `holderAddress`). They were vestigial, the Phase 2 reset drops them.
func InitializeTokenBalancesCollection() error {
	collection := configs.GetTokenBalancesCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing tokenBalances collection and indexes")

	indexes := []mongo.IndexModel{
		{
			// Phase 2 primary key: per-(contract, holder, tokenID) ownership.
			// Legacy ERC-20 rows (no tokenID field) collapse to a null third
			// key, which the prior (contract, holder) unique already kept
			// distinct, so this is non-disruptive on existing data.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "holderAddress", Value: 1},
				{Key: "tokenID", Value: 1},
			},
			Options: options.Index().
				SetName("contract_holder_tokenID_idx").
				SetUnique(true).
				SetBackground(true),
		},
		{
			// Per-id holder lookups: powers /token/<addr>/holders?tokenID=.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "tokenID", Value: 1},
				{Key: "holderAddress", Value: 1},
			},
			Options: options.Index().
				SetName("contract_tokenID_holder_idx").
				SetBackground(true),
		},
		{
			// Address-page sweep: list every token row a holder owns.
			Keys: bson.D{
				{Key: "holderAddress", Value: 1},
			},
			Options: options.Index().SetName("holder_idx"),
		},
		{
			// Token-page sweep: list every holder/row for a contract.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
			},
			Options: options.Index().SetName("contract_idx"),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		configs.Logger.Error("Failed to create indexes for token balances",
			zap.Error(err))
		return err
	}

	// Drop the prior 2-tuple unique once the new 3-tuple unique is online.
	// We can't rely on the legacy index name because different MongoDB
	// versions / manual creators may have named it differently (the
	// `configs.ConnectDB` path historically created it without an explicit
	// SetName, so Mongo auto-generated something like
	// `contractAddress_1_holderAddress_1`, but that's not a guarantee).
	// Iterate every index on the collection, match on the key spec instead,
	// and drop any (contractAddress, holderAddress) unique. The new 3-tuple
	// unique is keyed on three fields so it can't be confused.
	if cursor, err := collection.Indexes().List(ctx); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var idx struct {
				Name   string `bson:"name"`
				Key    bson.D `bson:"key"`
				Unique bool   `bson:"unique"`
			}
			if decErr := cursor.Decode(&idx); decErr != nil {
				continue
			}
			if len(idx.Key) != 2 {
				continue
			}
			// Match on the (contractAddress, holderAddress) key pair, in
			// either order, regardless of direction. The legacy unique
			// always pinned both to ascending (1), but matching by name
			// only would miss any variant.
			haveContract := false
			haveHolder := false
			for _, e := range idx.Key {
				switch e.Key {
				case "contractAddress":
					haveContract = true
				case "holderAddress":
					haveHolder = true
				}
			}
			if !haveContract || !haveHolder {
				continue
			}
			if _, dErr := collection.Indexes().DropOne(ctx, idx.Name); dErr != nil {
				msg := dErr.Error()
				if !strings.Contains(msg, "IndexNotFound") &&
					!strings.Contains(msg, "ns does not exist") &&
					!strings.Contains(msg, "NamespaceNotFound") {
					configs.Logger.Warn("Could not drop legacy 2-tuple unique; new 3-tuple unique still active",
						zap.String("indexName", idx.Name),
						zap.Error(dErr))
				}
			} else {
				configs.Logger.Info("Dropped legacy 2-tuple tokenBalances index",
					zap.String("indexName", idx.Name))
			}
		}
	} else {
		configs.Logger.Warn("Could not list tokenBalances indexes for migration; new 3-tuple unique still active",
			zap.Error(err))
	}

	// Also retire the vestigial pre-Phase-2 indexes that targeted a non-
	// existent `address` field. Safe to drop, the new indexes cover the
	// same access patterns through `holderAddress`.
	for _, name := range []string{"contract_address_idx", "address_idx"} {
		if _, err := collection.Indexes().DropOne(ctx, name); err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "IndexNotFound") &&
				!strings.Contains(msg, "ns does not exist") &&
				!strings.Contains(msg, "NamespaceNotFound") {
				configs.Logger.Warn("Could not drop vestigial tokenBalances index",
					zap.String("indexName", name),
					zap.Error(err))
			}
		}
	}

	configs.Logger.Info("Successfully initialized tokenBalances collection and indexes")
	return nil
}
