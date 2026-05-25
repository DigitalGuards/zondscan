package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreTokenTransfer stores a token transfer event in the database
func StoreTokenTransfer(transfer models.TokenTransfer) error {
	// Get explicit reference to the tokenTransfers collection
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Additional validation and normalization before inserting
	if transfer.From == "" {
		transfer.From = configs.QRLZeroAddress // Normalize empty from address to zero address
	}

	if transfer.To == "" {
		transfer.To = configs.QRLZeroAddress // Normalize empty to address to zero address
	}

	// Normalize addresses to canonical Q-prefix form
	transfer.From = validation.ConvertToQAddress(transfer.From)
	transfer.To = validation.ConvertToQAddress(transfer.To)
	transfer.ContractAddress = validation.ConvertToQAddress(transfer.ContractAddress)

	// Debug-level log for per-record operations; Info is reserved for batch summaries.
	configs.Logger.Debug("Inserting token transfer document",
		zap.String("token", transfer.TokenSymbol),
		zap.String("from", transfer.From),
		zap.String("to", transfer.To),
		zap.String("txHash", transfer.TxHash))

	_, err := collection.InsertOne(ctx, transfer)
	if err != nil {
		configs.Logger.Error("Failed to store token transfer",
			zap.String("txHash", transfer.TxHash),
			zap.String("token", transfer.TokenSymbol),
			zap.Error(err))
		return err
	}

	configs.Logger.Debug("Successfully stored token transfer in database",
		zap.String("token", transfer.TokenSymbol),
		zap.String("txHash", transfer.TxHash))
	return nil
}

// GetTokenTransfersByContract retrieves all transfers for a specific token contract
func GetTokenTransfersByContract(contractAddress string, skip, limit int64) ([]models.TokenTransfer, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := collection.Find(ctx,
		bson.M{"contractAddress": contractAddress},
		opts,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err = cursor.All(ctx, &transfers); err != nil {
		return nil, err
	}

	return transfers, nil
}

// GetTokenTransfersByAddress retrieves all transfers involving a specific address (as sender or receiver)
func GetTokenTransfersByAddress(address string, skip, limit int64) ([]models.TokenTransfer, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := collection.Find(ctx,
		bson.M{
			"$or": []bson.M{
				{"from": address},
				{"to": address},
			},
		},
		opts,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err = cursor.All(ctx, &transfers); err != nil {
		return nil, err
	}

	return transfers, nil
}

// TokenTransferExists checks if a specific token transfer is already
// persisted. The dedupe key is (txHash, contractAddress, logIndex, tokenID):
//
//   - logIndex was added in #88 so multi-transfer txs (a DEX swap that
//     emits 3 Transfer events) don't collide on (txHash, contract) alone.
//   - tokenID was added with NFT support: ERC-1155 TransferBatch logs
//     produce N rows per log (one per (id, value) tuple) that share the
//     same (txHash, contract, logIndex), only tokenID distinguishes them.
//
// Legacy rows pre-date the LogIndex and TokenID fields, so a missing-field
// match is needed for the empty-string sentinel paths (direct-calldata
// writes LogIndex="" with TokenID=""; pre-NFT rows have no tokenID at all).
func TokenTransferExists(txHash, contractAddress, logIndex, tokenID string) (bool, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"txHash":          txHash,
		"contractAddress": contractAddress,
	}

	andClauses := []bson.M{}
	if logIndex == "" {
		andClauses = append(andClauses, bson.M{"$or": []bson.M{
			{"logIndex": ""},
			{"logIndex": bson.M{"$exists": false}},
		}})
	} else {
		filter["logIndex"] = logIndex
	}
	if tokenID == "" {
		// `omitempty` on the new TokenID field strips it for ERC-20 writes,
		// and pre-NFT rows never had the field, both must match this clause.
		andClauses = append(andClauses, bson.M{"$or": []bson.M{
			{"tokenID": ""},
			{"tokenID": bson.M{"$exists": false}},
		}})
	} else {
		filter["tokenID"] = tokenID
	}
	if len(andClauses) > 0 {
		filter["$and"] = andClauses
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to check if token transfer exists",
			zap.String("txHash", txHash),
			zap.String("contractAddress", contractAddress),
			zap.String("logIndex", logIndex),
			zap.String("tokenID", tokenID),
			zap.Error(err))
		return false, err
	}

	return count > 0, nil
}

// ProcessBlockTokenTransfers processes all token-transfer events in a block
// across the three supported standards (ERC-20, ERC-721, ERC-1155).
//
// We pull all three event signatures in a single qrl_getLogs call (OR-matched
// on topic[0]), classify the emitting contract via supportsInterface, then
// dispatch to a per-standard decoder. ERC-20/721 share the same topic[0] so
// the (standard, topicCount) tuple is the disambiguator, relying on the
// contract's persisted classification rather than per-event guesswork
// closes the topic-0 ambiguity surface.
func ProcessBlockTokenTransfers(blockNumber string, blockTimestamp string) error {
	sigs := []string{
		rpc.TransferEventSignature,       // ERC-20 + ERC-721
		rpc.TransferSingleEventSignature, // ERC-1155
		rpc.TransferBatchEventSignature,  // ERC-1155
	}

	configs.Logger.Info("Searching for token transfers",
		zap.String("blockNumber", blockNumber),
		zap.Strings("eventSignatures", sigs))

	response, err := rpc.ZondGetBlockLogs(blockNumber, sigs)
	if err != nil {
		configs.Logger.Error("Failed to get logs for block",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return err
	}

	if response == nil || len(response.Result) == 0 {
		configs.Logger.Debug("No token transfer logs found in block",
			zap.String("blockNumber", blockNumber))
		return nil
	}

	configs.Logger.Info("Found potential token transfer logs",
		zap.String("blockNumber", blockNumber),
		zap.Int("logCount", len(response.Result)))

	tokenTransfersFound := 0
	for _, log := range response.Result {
		if len(log.Topics) < 3 {
			configs.Logger.Debug("Skipping log with insufficient topics",
				zap.String("txHash", log.TransactionHash),
				zap.Int("topicCount", len(log.Topics)))
			continue
		}

		contractAddress := log.Address
		contract, standard := EnsureContractClassified(contractAddress, blockNumber, log.TransactionHash)
		if standard == "" || contract == nil {
			configs.Logger.Debug("Contract is not a recognised token standard, skipping",
				zap.String("address", contractAddress))
			continue
		}

		rows, decErr := decodeTransferLog(log, contract, standard, blockNumber, blockTimestamp)
		if decErr != nil {
			configs.Logger.Warn("Failed to decode transfer log",
				zap.String("txHash", log.TransactionHash),
				zap.String("contract", contractAddress),
				zap.String("standard", standard),
				zap.String("topic0", log.Topics[0]),
				zap.Int("topicCount", len(log.Topics)),
				zap.Error(decErr))
			continue
		}

		for _, row := range rows {
			exists, err := TokenTransferExists(row.TxHash, row.ContractAddress, row.LogIndex, row.TokenID)
			if err != nil {
				configs.Logger.Error("Failed to check token transfer dedup",
					zap.String("txHash", row.TxHash),
					zap.Error(err))
				continue
			}
			if exists {
				configs.Logger.Debug("Skipping duplicate token transfer",
					zap.String("txHash", row.TxHash),
					zap.String("logIndex", row.LogIndex),
					zap.String("tokenID", row.TokenID))
				continue
			}

			if err := StoreTokenTransfer(row); err != nil {
				configs.Logger.Error("Failed to store token transfer",
					zap.String("txHash", row.TxHash),
					zap.Error(err))
				continue
			}
			tokenTransfersFound++

			// Per-standard balance maintenance.
			//
			// ERC-20: refresh sender + recipient via the existing helper
			// (RPC-confirmed balanceOf, full-precision string storage).
			//
			// ERC-721: the row's transfer ALREADY happened on chain by the
			// time we got the log. ownerOf is global truth, so a single
			// StoreERC721Ownership call refreshes the (contract, tokenID)
			// row to point at the new holder AND clears the prior holder's
			// row through the single-owner-invariant branch.
			//
			// ERC-1155: refresh both sides of the transfer with balanceOf
			// (per id). Zero balance deletes the row (sparse storage).
			// Errors are logged but non-fatal, the underlying tokenTransfer
			// row is already persisted, balance refresh is reconcilable
			// next time the holder transacts.
			switch standard {
			case rpc.StandardERC20:
				if err := StoreTokenBalance(contractAddress, row.From, row.Amount, blockNumber); err != nil {
					configs.Logger.Error("Failed to update sender token balance",
						zap.String("address", row.From),
						zap.String("contractAddress", contractAddress),
						zap.Error(err))
				}
				if err := StoreTokenBalance(contractAddress, row.To, row.Amount, blockNumber); err != nil {
					configs.Logger.Error("Failed to update recipient token balance",
						zap.String("address", row.To),
						zap.String("contractAddress", contractAddress),
						zap.Error(err))
				}
			case rpc.StandardERC721:
				if id, ok := new(big.Int).SetString(row.TokenID, 10); ok {
					if err := StoreERC721Ownership(contractAddress, id, blockNumber); err != nil {
						configs.Logger.Warn("ERC-721 ownership refresh failed; will retry on next transfer",
							zap.String("contractAddress", contractAddress),
							zap.String("tokenID", row.TokenID),
							zap.Error(err))
					}
					// Phase 3b: $setOnInsert a stub so the metadata fetcher
					// finds this token on its next poll. Hot path; failure
					// is non-fatal, the row already lives in tokenTransfers.
					if err := StubTokenMetadata(context.Background(), contractAddress, row.TokenID, rpc.StandardERC721); err != nil {
						configs.Logger.Debug("ERC-721 metadata stub upsert failed",
							zap.String("contractAddress", contractAddress),
							zap.String("tokenID", row.TokenID),
							zap.Error(err))
					}
				} else {
					configs.Logger.Warn("ERC-721 transfer row has unparseable tokenID; skipping ownership refresh",
						zap.String("contractAddress", contractAddress),
						zap.String("tokenID", row.TokenID))
				}
			case rpc.StandardERC1155:
				if id, ok := new(big.Int).SetString(row.TokenID, 10); ok {
					if err := StoreERC1155Balance(contractAddress, row.From, id, blockNumber); err != nil {
						configs.Logger.Warn("ERC-1155 sender balance refresh failed",
							zap.String("contractAddress", contractAddress),
							zap.String("holder", row.From),
							zap.String("tokenID", row.TokenID),
							zap.Error(err))
					}
					if err := StoreERC1155Balance(contractAddress, row.To, id, blockNumber); err != nil {
						configs.Logger.Warn("ERC-1155 recipient balance refresh failed",
							zap.String("contractAddress", contractAddress),
							zap.String("holder", row.To),
							zap.String("tokenID", row.TokenID),
							zap.Error(err))
					}
					if err := StubTokenMetadata(context.Background(), contractAddress, row.TokenID, rpc.StandardERC1155); err != nil {
						configs.Logger.Debug("ERC-1155 metadata stub upsert failed",
							zap.String("contractAddress", contractAddress),
							zap.String("tokenID", row.TokenID),
							zap.Error(err))
					}
				} else {
					configs.Logger.Warn("ERC-1155 transfer row has unparseable tokenID; skipping balance refresh",
						zap.String("contractAddress", contractAddress),
						zap.String("tokenID", row.TokenID))
				}
			}
		}
	}

	configs.Logger.Info("Finished processing token transfers",
		zap.String("blockNumber", blockNumber),
		zap.Int("transfersProcessed", tokenTransfersFound))

	return nil
}

// decodeTransferLog dispatches a transfer log to the correct per-standard
// decoder. The (standard, topic0, len(topics)) triple disambiguates the
// ERC-20/ERC-721 topic-0 collision without ever guessing, the contract's
// persisted classification (from supportsInterface) is the source of truth.
//
// Returns a non-nil error only on truly mismatched shapes (e.g. a 4-topic
// Transfer on a contract we classified as ERC-20). Decoder helpers
// surface their own errors for malformed data.
func decodeTransferLog(
	log models.Log,
	contract *models.ContractInfo,
	standard, blockNumber, blockTimestamp string,
) ([]models.TokenTransfer, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	topic0 := log.Topics[0]

	base := models.TokenTransfer{
		ContractAddress: log.Address,
		BlockNumber:     blockNumber,
		TxHash:          log.TransactionHash,
		LogIndex:        log.LogIndex,
		Timestamp:       blockTimestamp,
		TokenSymbol:     contract.Symbol,
		TokenDecimals:   contract.Decimals,
		TokenName:       contract.Name,
		TransferType:    "event",
		TokenStandard:   standard,
	}

	switch {
	case standard == rpc.StandardERC20 && topic0 == rpc.TransferEventSignature && len(log.Topics) == 3:
		return decodeERC20TransferRow(log, base)
	case standard == rpc.StandardERC721 && topic0 == rpc.TransferEventSignature && len(log.Topics) == 4:
		return decodeERC721TransferRow(log, base)
	case standard == rpc.StandardERC1155 && topic0 == rpc.TransferSingleEventSignature && len(log.Topics) == 4:
		return decodeERC1155TransferSingleRow(log, base)
	case standard == rpc.StandardERC1155 && topic0 == rpc.TransferBatchEventSignature && len(log.Topics) == 4:
		return decodeERC1155TransferBatchRows(log, base)
	default:
		return nil, fmt.Errorf("standard/topic mismatch: standard=%s topic0=%s topicCount=%d",
			standard, topic0, len(log.Topics))
	}
}

// decodeERC20TransferRow produces a single TokenTransfer for an ERC-20
// Transfer log. Amount is left as the raw hex `log.Data` for backward
// compatibility with rows written before Phase 1.
func decodeERC20TransferRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, _, err := ParseStandardTransferTopics(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = log.Data
	return []models.TokenTransfer{base}, nil
}

// decodeERC721TransferRow produces a single TokenTransfer for an ERC-721
// Transfer log (4 topics, tokenID in topic[3], data empty).
// Amount is the canonical "1", every ERC-721 transfer moves exactly one
// token, so aggregate "transfers moved" queries stay consistent.
func decodeERC721TransferRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, tokenID, err := rpc.ParseERC721Transfer(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = "1"
	base.TokenID = tokenID.String()
	return []models.TokenTransfer{base}, nil
}

// decodeERC1155TransferSingleRow produces a single TokenTransfer for an
// ERC-1155 TransferSingle log. Amount is the decimal-string value (not hex)
// since the data field carries a proper uint256 that we can normalise.
func decodeERC1155TransferSingleRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, id, value, err := rpc.ParseERC1155TransferSingle(log)
	if err != nil {
		return nil, err
	}
	base.From = normalizeAddress(from)
	base.To = normalizeAddress(to)
	base.Amount = value.String()
	base.TokenID = id.String()
	return []models.TokenTransfer{base}, nil
}

// decodeERC1155TransferBatchRows fans a single TransferBatch log out to N
// rows, one per (id, value) tuple. The compound unique index
// (txHash, contract, logIndex, tokenID) keeps the rows distinct on replay.
func decodeERC1155TransferBatchRows(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, ids, values, err := rpc.ParseERC1155TransferBatch(log)
	if err != nil {
		return nil, err
	}
	fromQ := normalizeAddress(from)
	toQ := normalizeAddress(to)
	out := make([]models.TokenTransfer, len(ids))
	for i := range ids {
		row := base
		row.From = fromQ
		row.To = toQ
		row.Amount = values[i].String()
		row.TokenID = ids[i].String()
		out[i] = row
	}
	return out, nil
}

// ParseStandardTransferTopics extracts (from, to) from a standard Transfer
// event's topics[1] / topics[2]. Returns the canonical Q-prefix lowercase
// form and an empty *big.Int placeholder (kept in the signature so future
// callers can fold in amount parsing if needed). Errors on malformed topics.
func ParseStandardTransferTopics(log models.Log) (string, string, *big.Int, error) {
	if len(log.Topics) < 3 {
		return "", "", nil, fmt.Errorf("transfer log requires >=3 topics, got %d", len(log.Topics))
	}
	from, err := extractAddressTopic(log.Topics[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("from: %w", err)
	}
	to, err := extractAddressTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, fmt.Errorf("to: %w", err)
	}
	return from, to, new(big.Int), nil
}

// extractAddressTopic returns the Q-prefix lowercase address from a
// 32-byte indexed topic. Tolerates topics of length <66 (some legacy
// implementations strip leading zeros).
func extractAddressTopic(topic string) (string, error) {
	stripped := topic
	if len(stripped) >= 2 && stripped[:2] == "0x" {
		stripped = stripped[2:]
	}
	if len(stripped) < 40 {
		return "", fmt.Errorf("topic too short: %s", topic)
	}
	addr := "Q" + strings.ToLower(stripped[len(stripped)-40:])
	if !validation.IsValidAddress(addr) {
		return "", fmt.Errorf("invalid address derived from topic: %s", addr)
	}
	return addr, nil
}

// normalizeAddress maps degenerate empty / bare-Q forms to the canonical
// zero address; passes valid Q-addresses through unchanged.
func normalizeAddress(addr string) string {
	if addr == "" || addr == "q" || addr == "Q" {
		return configs.QRLZeroAddress
	}
	return addr
}

// dropLegacyConflictingIndexes scans `collection`'s indexes and drops any
// whose key spec matches one of `targets`, regardless of the index name.
// Used to migrate off auto-named indexes (Mongo's default `field_1_field_1`)
// before re-creating the same key spec under a stable name. Without this
// step Mongo rejects the second create with IndexOptionsConflict and aborts
// the whole CreateMany batch.
//
// Key match is direction-aware and order-aware (the `bson.D` is a slice).
// Pass exactly the key spec you intend to recreate.
func dropLegacyConflictingIndexes(ctx context.Context, collection *mongo.Collection, targets []bson.D) {
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		configs.Logger.Warn("Could not list indexes for legacy cleanup; existing autoindexes may cause IndexOptionsConflict",
			zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var idx struct {
			Name string `bson:"name"`
			Key  bson.D `bson:"key"`
		}
		if decErr := cursor.Decode(&idx); decErr != nil {
			continue
		}
		if idx.Name == "_id_" {
			continue
		}
		for _, want := range targets {
			if !sameKeySpec(idx.Key, want) {
				continue
			}
			if _, dErr := collection.Indexes().DropOne(ctx, idx.Name); dErr != nil {
				msg := dErr.Error()
				if !strings.Contains(msg, "IndexNotFound") &&
					!strings.Contains(msg, "ns does not exist") &&
					!strings.Contains(msg, "NamespaceNotFound") {
					configs.Logger.Warn("Could not drop legacy conflicting index",
						zap.String("indexName", idx.Name),
						zap.Error(dErr))
				}
			} else {
				configs.Logger.Info("Dropped legacy conflicting index",
					zap.String("indexName", idx.Name))
			}
			break
		}
	}
}

// sameKeySpec returns true when two index key specs are identical in field
// order, name, and direction. Mongo treats `{a:1,b:1}` and `{b:1,a:1}` as
// different indexes, so order matters for collision detection.
func sameKeySpec(a, b bson.D) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key {
			return false
		}
		if a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

// InitializeTokenTransfersCollection ensures the token transfers collection is set up with proper indexes.
// Uses CreateMany which is a no-op for indexes that already exist, safe to call on every restart.
func InitializeTokenTransfersCollection() error {
	collection := configs.GetTokenTransfersCollection()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs.Logger.Info("Initializing tokenTransfers collection and indexes")

	// Drop the long-gone txHash-only unique (pre-#88). DropOne returns
	// IndexNotFound on fresh deployments, acceptable.
	if _, err := collection.Indexes().DropOne(ctx, "txHash_idx"); err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "IndexNotFound") &&
			!strings.Contains(msg, "ns does not exist") &&
			!strings.Contains(msg, "NamespaceNotFound") {
			configs.Logger.Warn("Could not drop legacy txHash_idx",
				zap.Error(err))
		}
	}

	// Drop legacy auto-named indexes that collide on key spec with the
	// named ones we're about to create. configs/setup.go used to register
	// these without SetName(), so Mongo gave them auto-generated names
	// like "contractAddress_1_blockNumber_1" / "txHash_1". When the named
	// versions below are later submitted via CreateMany, Mongo rejects
	// the whole batch with IndexOptionsConflict — including the new
	// 4-tuple unique that ERC-1155 TransferBatch + ERC-721 batch mint
	// depend on. Same pattern as the tokenBalances migration further
	// down: match by key spec (not name), drop everything that conflicts
	// with what we're about to (re)create. Crucially this includes the
	// legacy "txHash_1" unique-on-txHash, which would otherwise reject
	// every Transfer event after the first per tx (E11000 duplicate-key).
	// See https://github.com/DigitalGuards/zondscan/pull/110 for the
	// incident write-up.
	dropLegacyConflictingIndexes(ctx, collection,
		[]bson.D{
			{{Key: "contractAddress", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "from", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "to", Value: 1}, {Key: "blockNumber", Value: 1}},
			{{Key: "txHash", Value: 1}},
		},
	)

	// Create the new 4-tuple unique index BEFORE dropping the old 3-tuple
	// version. Order matters: if creation fails (duplicate-key on legacy
	// data, etc.) we want to leave the old unique in place; only after
	// the new one is online do we drop the old.
	//
	// Legacy rows (pre-tokenID) lack the field, Mongo treats missing
	// fields as `null` for index uniqueness. Since the prior 3-tuple
	// unique guaranteed at most one (txHash, contract, logIndex) row,
	// the 4-tuple `(txHash, contract, logIndex, null)` is also unique
	// by extension, no migration collision on existing data.
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("contract_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("from_block_idx"),
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().SetName("to_block_idx"),
		},
		{
			// Phase 1 unique compound: (txHash, contract, logIndex, tokenID).
			// Replaces the 3-tuple unique with the same race-condition floor
			// while keeping ERC-1155 TransferBatch rows distinct on tokenID.
			Keys: bson.D{
				{Key: "txHash", Value: 1},
				{Key: "contractAddress", Value: 1},
				{Key: "logIndex", Value: 1},
				{Key: "tokenID", Value: 1},
			},
			Options: options.Index().
				SetName("txHash_contract_logIndex_tokenID_idx").
				SetUnique(true).
				SetBackground(true),
		},
		{
			// Per-NFT history queries: list all transfers of a specific
			// tokenID in chronological order. Non-unique; serves the
			// per-NFT page (Phase 3) and the holder timeline UI.
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "tokenID", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
			Options: options.Index().
				SetName("contract_tokenID_block_idx").
				SetBackground(true),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		configs.Logger.Error("Failed to create token transfer indexes",
			zap.Error(err))
		return err
	}

	// Now that the new unique is online, retire the prior 3-tuple unique
	// from #88. Missing on fresh deployments, IndexNotFound is fine.
	if _, err := collection.Indexes().DropOne(ctx, "txHash_contract_logIndex_idx"); err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "IndexNotFound") &&
			!strings.Contains(msg, "ns does not exist") &&
			!strings.Contains(msg, "NamespaceNotFound") {
			configs.Logger.Warn("Could not drop legacy txHash_contract_logIndex_idx; new 4-tuple unique still active",
				zap.Error(err))
		}
	}

	configs.Logger.Info("Successfully initialized tokenTransfers collection and indexes")
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
