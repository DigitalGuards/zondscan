package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"QRL2MongoDB/validation"
	"context"
	"fmt"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	// Populate the numeric block number so sort operations order correctly.
	// BlockNumber is the raw hex string; utils.HexToInt64 returns 0 on any
	// parse error.
	transfer.BlockNumberInt = utils.HexToInt64(transfer.BlockNumber)

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
// match is needed for the empty-string sentinel paths (rows written by the
// long-dead direct-calldata path carry LogIndex="" with TokenID=""; pre-NFT
// rows have no tokenID at all).
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
// compatibility with rows written before Phase 1 (the parsed amount from
// ParseTransferEvent is deliberately unused).
func decodeERC20TransferRow(log models.Log, base models.TokenTransfer) ([]models.TokenTransfer, error) {
	from, to, _, err := rpc.ParseTransferEvent(log)
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

// normalizeAddress maps degenerate empty / bare-Q forms to the canonical
// zero address; passes valid Q-addresses through unchanged.
func normalizeAddress(addr string) string {
	if addr == "" || addr == "q" || addr == "Q" {
		return configs.QRLZeroAddress
	}
	return addr
}
