package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"fmt"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenBalance updates the token balance for a given address
func StoreTokenBalance(contractAddress string, holderAddress string, amount string, blockNumber string) error {
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
	if holderAddress == "Q0" ||
		holderAddress == configs.QRLZeroAddress ||
		holderAddress == "0x0" ||
		holderAddress == "0x0000000000000000000000000000000000000000" {
		configs.Logger.Debug("Skipping token balance update for zero address",
			zap.String("holderAddress", holderAddress))
		return nil
	}

	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		configs.Logger.Error("Failed to get token balances collection")
		return fmt.Errorf("token balances collection is nil")
	}

	// Get current balance from RPC with more robust error handling
	configs.Logger.Debug("Calling RPC to get current token balance")
	balance, err := rpc.GetTokenBalance(contractAddress, holderAddress)
	if err != nil {
		configs.Logger.Error("Failed to get token balance from RPC",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		// Continue with a zero balance if we can't get the actual balance
		// This allows us to at least record that we tried to update this token balance
		configs.Logger.Debug("Using default zero balance after RPC failure")
		balance = "0"
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
	update := bson.M{
		"$set": bson.M{
			"contractAddress": contractAddress,
			"holderAddress":   holderAddress,
			"balance":         balance,
			"blockNumber":     blockNumber,
			"updatedAt":       time.Now().UTC().Format(time.RFC3339),
			"tokenStandard":   rpc.StandardERC20,
		},
	}

	// Update options
	opts := options.Update().SetUpsert(true)

	// Filter to find existing document
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform upsert
	result, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to update token balance in database",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress),
			zap.Error(err))
		return fmt.Errorf("failed to update token balance: %v", err)
	}

	configs.Logger.Debug("Token balance update completed",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress),
		zap.Int64("matchedCount", result.MatchedCount),
		zap.Int64("modifiedCount", result.ModifiedCount),
		zap.Int64("upsertedCount", result.UpsertedCount))

	return nil
}

// GetTokenBalance retrieves the current token balance for a holder
func GetTokenBalance(contractAddress string, holderAddress string) (*models.TokenBalance, error) {
	collection := configs.GetTokenBalancesCollection()
	var balance models.TokenBalance

	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, filter).Decode(&balance)
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

// GetTokenHolders retrieves all holders of a specific token
func GetTokenHolders(contractAddress string) ([]models.TokenBalance, error) {
	collection := configs.GetTokenBalancesCollection()
	var balances []models.TokenBalance

	filter := bson.M{"contractAddress": contractAddress}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	err = cursor.All(ctx, &balances)
	if err != nil {
		return nil, err
	}

	return balances, nil
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
	if tokenID == nil {
		return fmt.Errorf("tokenID required")
	}
	contractAddress = validation.ConvertToQAddress(contractAddress)

	configs.Logger.Debug("Refreshing ERC-721 ownership",
		zap.String("contract", contractAddress),
		zap.String("tokenID", tokenID.String()),
		zap.String("blockNumber", blockNumber))

	owner, err := rpc.GetERC721Owner(contractAddress, tokenID)
	if err != nil {
		// Transport / unmarshal failure. Preserve existing state.
		configs.Logger.Warn("GetERC721Owner transport error; preserving existing balance state",
			zap.String("contract", contractAddress),
			zap.String("tokenID", tokenID.String()),
			zap.Error(err))
		return err
	}

	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		return fmt.Errorf("token balances collection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	idStr := tokenID.String()

	// No current owner (burned / never minted): drop any stale row.
	if owner == "" {
		_, dErr := collection.DeleteMany(ctx, bson.M{
			"contractAddress": contractAddress,
			"tokenID":         idStr,
			"tokenStandard":   rpc.StandardERC721,
		})
		if dErr != nil {
			configs.Logger.Warn("Failed to delete stale ERC-721 ownership row",
				zap.String("contract", contractAddress),
				zap.String("tokenID", idStr),
				zap.Error(dErr))
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
		configs.Logger.Warn("Failed to delete stale ERC-721 ownership row(s) for prior holder",
			zap.String("contract", contractAddress),
			zap.String("tokenID", idStr),
			zap.Error(dErr))
	}

	// Upsert the current owner's row.
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   owner,
		"tokenID":         idStr,
	}
	update := bson.M{
		"$set": bson.M{
			"contractAddress": contractAddress,
			"holderAddress":   owner,
			"tokenID":         idStr,
			"tokenStandard":   rpc.StandardERC721,
			"balance":         "1",
			"blockNumber":     blockNumber,
			"updatedAt":       time.Now().UTC().Format(time.RFC3339),
		},
	}
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
	if tokenID == nil {
		return fmt.Errorf("tokenID required")
	}
	contractAddress = validation.ConvertToQAddress(contractAddress)
	holderAddress = validation.ConvertToQAddress(holderAddress)

	// Skip zero-address; mint/burn endpoints don't hold balances.
	if holderAddress == "Q0" ||
		holderAddress == configs.QRLZeroAddress ||
		holderAddress == "0x0" ||
		holderAddress == "0x0000000000000000000000000000000000000000" {
		return nil
	}

	configs.Logger.Debug("Refreshing ERC-1155 balance",
		zap.String("contract", contractAddress),
		zap.String("holder", holderAddress),
		zap.String("tokenID", tokenID.String()),
		zap.String("blockNumber", blockNumber))

	balance, err := rpc.GetERC1155Balance(contractAddress, holderAddress, tokenID)
	if err != nil {
		configs.Logger.Warn("GetERC1155Balance transport error; preserving existing balance state",
			zap.String("contract", contractAddress),
			zap.String("holder", holderAddress),
			zap.String("tokenID", tokenID.String()),
			zap.Error(err))
		return err
	}

	collection := configs.GetTokenBalancesCollection()
	if collection == nil {
		return fmt.Errorf("token balances collection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	idStr := tokenID.String()
	filter := bson.M{
		"contractAddress": contractAddress,
		"holderAddress":   holderAddress,
		"tokenID":         idStr,
	}

	// Sparse storage: delete the row when balance is zero.
	if balance == nil || balance.Sign() == 0 {
		if _, dErr := collection.DeleteOne(ctx, filter); dErr != nil && dErr != mongo.ErrNoDocuments {
			configs.Logger.Warn("Failed to delete zero ERC-1155 balance row",
				zap.String("contract", contractAddress),
				zap.String("holder", holderAddress),
				zap.String("tokenID", idStr),
				zap.Error(dErr))
		}
		return nil
	}

	update := bson.M{
		"$set": bson.M{
			"contractAddress": contractAddress,
			"holderAddress":   holderAddress,
			"tokenID":         idStr,
			"tokenStandard":   rpc.StandardERC1155,
			"balance":         balance.String(),
			"blockNumber":     blockNumber,
			"updatedAt":       time.Now().UTC().Format(time.RFC3339),
		},
	}
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
