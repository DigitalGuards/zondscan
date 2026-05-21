package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StoreContract stores or merges contract information in the database.
//
// **Merge-safety invariant**: the syncer is the only writer of the
// syncer-owned fields enumerated by `syncerOwnedSet` below; the backend
// verify endpoint is the only writer of the verification fields
// (verified / sourceCode / abi / contractName / compilerVersion /
// optimizationEnabled / optimizationRuns / evmVersion /
// constructorArguments / libraries / license / verificationMethod /
// verifiedAt). The two write paths NEVER overlap.
//
// Previously this function did `$set: updateData` (whole-doc), which
// re-introduced a race: if the syncer's FindOne read predated a
// concurrent verification write, the whole-doc $set would clobber the
// freshly-written verification fields with stale empties from the
// in-memory copy. Switching to a field-scoped $set listing only the
// syncer-owned fields makes that race impossible by construction —
// the syncer literally cannot touch verification keys.
func StoreContract(contract models.ContractInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Normalize addresses to canonical Q-prefix form
	contract.Address = validation.ConvertToQAddress(contract.Address)
	contract.CreatorAddress = validation.ConvertToQAddress(contract.CreatorAddress)

	collection := configs.GetContractsCollection()
	filter := bson.M{"address": contract.Address}

	// Attempt to find existing contract document
	var existingContract models.ContractInfo
	err := collection.FindOne(ctx, filter).Decode(&existingContract)

	merged := contract

	if err == nil {
		// Existing contract found, merge new data into it
		configs.Logger.Debug("Found existing contract, merging data", zap.String("address", contract.Address))
		merged = existingContract

		// Merge fields from the new 'contract' object, only if the new value is non-empty/non-zero
		// and the existing value *is* empty/zero. This prioritizes data from the creation tx.
		// Treat bare "Q" (from legacy ConvertToQAddress("")) as empty.
		if (merged.CreatorAddress == "" || merged.CreatorAddress == "Q") && contract.CreatorAddress != "" && contract.CreatorAddress != "Q" {
			merged.CreatorAddress = contract.CreatorAddress
		}
		if merged.CreationTransaction == "" && contract.CreationTransaction != "" {
			merged.CreationTransaction = contract.CreationTransaction
		}
		if merged.CreationBlockNumber == "" && contract.CreationBlockNumber != "" {
			merged.CreationBlockNumber = contract.CreationBlockNumber
		}
		if merged.ContractCode == "" && contract.ContractCode != "" && contract.ContractCode != "0x" {
			merged.ContractCode = contract.ContractCode
		}
		if merged.Status == "" && contract.Status != "" {
			merged.Status = contract.Status
		} else if contract.Status != "" && merged.Status != contract.Status {
			merged.Status = contract.Status
		}

		// Token classification is promote-only: once a contract has been
		// identified as a token (Name/Symbol/Decimals populated from a
		// successful RPC probe), we never demote it back. The previous
		// logic clobbered Name/Symbol/Decimals/TotalSupply to empty whenever
		// `GetTokenInfo` returned `isToken=false` — and that path also
		// returns false on every *transient* RPC error (name()/symbol()
		// timeout, decode failure, node failover blip). Real-world symptom
		// was real ERC-20 tokens flickering to empty `name`/`symbol` in the
		// explorer during node hiccups, then restoring on the next
		// interaction that succeeded. Now the merge:
		//   - flips IsToken only false → true;
		//   - copies Name/Symbol/Decimals/TotalSupply only when the existing
		//     value is empty (fills gaps from a richer probe);
		//   - never clears non-empty token metadata.
		// Re-classification (true → false) is intentionally NOT a side
		// effect of any read path — it must be an explicit operator action.
		if contract.IsToken {
			merged.IsToken = true
			if merged.Name == "" && contract.Name != "" {
				merged.Name = contract.Name
			}
			if merged.Symbol == "" && contract.Symbol != "" {
				merged.Symbol = contract.Symbol
			}
			if merged.Decimals == 0 && contract.Decimals != 0 {
				merged.Decimals = contract.Decimals
			}
			if merged.TotalSupply == "" && contract.TotalSupply != "" {
				merged.TotalSupply = contract.TotalSupply
			}
		}
		// `contract.IsToken == false` is a no-op for IsToken + token
		// metadata; keep existing values intact.

		// TokenStandard promotion ladder: "" → ERC-20 → ERC-721/1155.
		// Same promote-only rationale as IsToken: a transient RPC blip
		// must never demote a previously-classified contract. ERC-1155
		// outranks ERC-721 so dual-impl edge cases (rare) pick the
		// broader standard. The merge never moves DOWN the ladder.
		if standardRank(contract.TokenStandard) > standardRank(merged.TokenStandard) {
			merged.TokenStandard = contract.TokenStandard
		}
		// HasERC165 latches true forever — a contract that ever responded
		// to supportsInterface didn't UN-implement ERC-165 later. Skip the
		// flag flip on transient probe failures (those return HasERC165=false).
		if contract.HasERC165 {
			merged.HasERC165 = true
		}
		// BaseURI is gap-fill only (Phase 3 will populate it from tokenURI
		// / uri probes). Never overwrite or clear.
		if merged.BaseURI == "" && contract.BaseURI != "" {
			merged.BaseURI = contract.BaseURI
		}
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		configs.Logger.Error("Failed to check for existing contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	// IsToken back-compat: any classified standard implies isToken=true so
	// the legacy `?isToken=true` API filter continues to surface NFT
	// collections alongside ERC-20s. Applied in BOTH the merge and the
	// fresh-write path — callers that set TokenStandard without IsToken
	// (e.g. backfill scripts) still produce correct rows.
	if merged.TokenStandard != "" {
		merged.IsToken = true
	}

	// Stamp updatedAt once for both code paths (merge + first-write).
	// Live indexing — the wall clock is the right source here.
	merged.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	opts := options.Update().SetUpsert(true)
	update := bson.M{"$set": syncerOwnedSet(merged)}

	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store/merge contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully stored/merged contract", zap.String("address", merged.Address))
	return nil
}

// syncerOwnedSet returns the $set payload for fields the syncer is allowed
// to write. Verification fields (verified / sourceCode / abi / ...) are
// deliberately absent — they belong exclusively to the backend verify
// endpoint. Keep this list in sync with the non-verification fields on
// models.ContractInfo; any field added to the model that the syncer also
// populates must be added here.
//
// We use an explicit allow-list rather than marshal-and-delete-known-keys
// for two reasons:
//
//  1. Default-deny is the safer posture for a write boundary protecting
//     trust-critical fields. Adding a new SYNCER-owned field and
//     forgetting to add it here results in a visible omission (easy to
//     spot in tests / data sampling). The inverted shape would default
//     to "syncer may write any new field" — a new VERIFICATION field
//     forgotten in the delete list would get silently clobbered. Silent
//     trust-store corruption is the much worse failure mode.
//  2. bson.Marshal + bson.Unmarshal per write is real overhead in the
//     hot path (every contract creation, every reprocess pass).
//
// The optional CustomERC20 caps respect their model `omitempty` semantics:
// empty values are not written, matching the pre-refactor on-disk shape so
// `$exists` queries (if any are ever added) behave the same.
func syncerOwnedSet(c models.ContractInfo) bson.M {
	m := bson.M{
		"address":             c.Address,
		"status":              c.Status,
		"isToken":             c.IsToken,
		"name":                c.Name,
		"symbol":              c.Symbol,
		"decimals":            c.Decimals,
		"totalSupply":         c.TotalSupply,
		"contractCode":        c.ContractCode,
		"creatorAddress":      c.CreatorAddress,
		"creationTransaction": c.CreationTransaction,
		"creationBlockNumber": c.CreationBlockNumber,
		"updatedAt":           c.UpdatedAt,
	}
	if c.MaxSupply != "" {
		m["maxSupply"] = c.MaxSupply
	}
	if c.MaxWalletAmount != "" {
		m["maxWalletAmount"] = c.MaxWalletAmount
	}
	if c.MaxTxLimit != "" {
		m["maxTxLimit"] = c.MaxTxLimit
	}
	// NFT classification — omit when empty/false so the document stays clean
	// for non-token contracts AND a transient blip that produces zero values
	// can't clobber a previously-set value (writing `false` would overwrite
	// `true`; omitting the key leaves the existing value untouched).
	if c.TokenStandard != "" {
		m["tokenStandard"] = c.TokenStandard
	}
	if c.HasERC165 {
		m["hasERC165"] = true
	}
	if c.BaseURI != "" {
		m["baseURI"] = c.BaseURI
	}
	return m
}

// standardRank orders TokenStandard values for promote-only merging.
// Higher rank = stricter / more specific classification. Promotions go
// up the ladder; demotions are silently dropped.
func standardRank(s string) int {
	switch s {
	case "ERC-1155":
		return 3
	case "ERC-721":
		return 2
	case "ERC-20":
		return 1
	default:
		return 0
	}
}

// GetContract retrieves contract information from the database
func GetContract(address string) (*models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)

	var contract models.ContractInfo
	err := configs.GetContractsCollection().FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %v", err)
	}

	return &contract, nil
}

// UpdateContractStatus updates the status of a contract
func UpdateContractStatus(address string, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"status": status}}
	_, err := configs.GetContractsCollection().UpdateOne(ctx, bson.M{"address": address}, update)
	if err != nil {
		return fmt.Errorf("failed to update contract status: %v", err)
	}

	return nil
}

// processContracts processes contract-related information from a transaction
func processContracts(tx *models.Transaction) (string, string, string, bool) {
	var to string
	var contractAddress string
	var statusTx string
	var isContract bool

	// Check if it's a contract creation transaction
	if tx.To == "" {
		// Get contract address and status from transaction receipt
		var err error
		contractAddress, statusTx, err = rpc.GetContractAddress(tx.Hash)
		if err != nil {
			configs.Logger.Error("Failed to get contract address",
				zap.String("hash", tx.Hash),
				zap.Error(err))
			return "", "", "", false
		}

		if contractAddress != "" {
			isContract = true

			// Get contract code
			contractCode, err := rpc.GetCode(contractAddress, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contractAddress),
					zap.Error(err))
			}

			// Classify the contract (ERC-20 / ERC-721 / ERC-1155 / unknown).
			// On transient probe failure the result is zero-valued; the
			// StoreContract merge's promote-only invariant prevents that
			// from clobbering a previously-good classification.
			detection, detErr := rpc.DetectContractType(contractAddress)
			if detErr != nil {
				configs.Logger.Warn("Contract type detection failed; storing without classification",
					zap.String("address", contractAddress),
					zap.Error(detErr))
			}

			// Store complete contract information
			contract := models.ContractInfo{
				Address:             contractAddress,
				Status:              statusTx,
				IsToken:             detection.Standard != "",
				Name:                detection.Name,
				Symbol:              detection.Symbol,
				Decimals:            detection.Decimals,
				TotalSupply:         detection.TotalSupply,
				TokenStandard:       detection.Standard,
				HasERC165:           detection.HasERC165,
				ContractCode:        contractCode,
				CreatorAddress:      tx.From,
				CreationTransaction: tx.Hash,
				CreationBlockNumber: tx.BlockNumber,
				UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
			}

			// Store the contract
			err = StoreContract(contract)
			if err != nil {
				configs.Logger.Error("Failed to store contract",
					zap.String("address", contractAddress),
					zap.Error(err))
			}
		}
	} else {
		to = tx.To
		statusTx = tx.Status

		// Check if the destination address is a contract
		isContract = IsAddressContract(to)
	}

	return to, contractAddress, statusTx, isContract
}

// IsAddressContract checks if an address is a contract by querying the contractCode collection
// and falling back to RPC getCode call if not found
func IsAddressContract(address string) bool {
	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)

	// First check our database
	contract := getContractFromDB(address)
	if contract != nil {
		return true
	}

	// If not in database, check via RPC
	code, err := rpc.GetCode(address, "latest")
	if err != nil {
		configs.Logger.Error("Failed to get code for address",
			zap.String("address", address),
			zap.Error(err))
		return false
	}

	// If code is not empty/0x, it's a contract
	isContract := code != "" && code != "0x" && code != "0x0"

	// If it's a contract, store it in our database
	if isContract {
		configs.Logger.Info("Detected existing contract",
			zap.String("address", address))

		// Classify (ERC-20 / 721 / 1155 / unknown). Transient probe
		// failures are logged but non-fatal — StoreContract preserves
		// any previously-good classification through its merge.
		detection, detErr := rpc.DetectContractType(address)
		if detErr != nil {
			configs.Logger.Warn("Contract type detection failed; storing without classification",
				zap.String("address", address),
				zap.Error(detErr))
		}

		// First try to get existing contract from both collections to preserve creation data
		existingContract, err := GetContract(address)

		// Create base contract info
		contract := models.ContractInfo{
			Address:       address,
			Status:        "0x1", // Assume successful
			IsToken:       detection.Standard != "",
			Name:          detection.Name,
			Symbol:        detection.Symbol,
			Decimals:      detection.Decimals,
			TotalSupply:   detection.TotalSupply,
			TokenStandard: detection.Standard,
			HasERC165:     detection.HasERC165,
			ContractCode:  code,
			UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		}

		// If we have existing contract data, preserve the creation information
		if err == nil && existingContract != nil {
			// Preserve creation information if present
			if existingContract.CreatorAddress != "" {
				contract.CreatorAddress = existingContract.CreatorAddress
			}
			if existingContract.CreationTransaction != "" {
				contract.CreationTransaction = existingContract.CreationTransaction
			}
			if existingContract.CreationBlockNumber != "" {
				contract.CreationBlockNumber = existingContract.CreationBlockNumber
			}
		}

		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to store detected contract",
				zap.String("address", address),
				zap.Error(err))
		}
	}

	return isContract
}

// getContractFromDB retrieves contract information from the contractCode collection
// Local version to avoid naming conflicts
func getContractFromDB(address string) *models.ContractInfo {
	// First check in the main contracts collection
	mainContract, err := GetContract(address)
	if err == nil && mainContract != nil {
		// If found in main collection, return it
		return mainContract
	}

	// If not found in main collection, check the contractCode collection
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = collection.FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
}

// ReprocessIncompleteContracts finds and updates contracts with missing information
func ReprocessIncompleteContracts() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find contracts with missing information, including bare "Q" creator addresses
	filter := bson.M{
		"$or": []bson.M{
			{"contractCode": ""},
			{"isToken": true, "totalSupply": ""},
			{"isToken": false, "name": "", "symbol": ""},
			{"creatorAddress": "Q"},
			{"creatorAddress": ""},
		},
	}

	cursor, err := configs.GetContractsCollection().Find(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to query incomplete contracts", zap.Error(err))
		return err
	}
	defer cursor.Close(ctx)

	var processedCount int
	for cursor.Next(ctx) {
		var contract models.ContractInfo
		if err := cursor.Decode(&contract); err != nil {
			configs.Logger.Error("Failed to decode contract", zap.Error(err))
			continue
		}

		// Store original creation information to ensure it's not lost
		creatorAddress := contract.CreatorAddress
		creationTransaction := contract.CreationTransaction
		creationBlockNumber := contract.CreationBlockNumber

		// Get contract code if missing
		if contract.ContractCode == "" {
			contractCode, err := rpc.GetCode(contract.Address, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contract.Address),
					zap.Error(err))
			} else {
				contract.ContractCode = contractCode
			}
		}

		// Get token information if missing. ReprocessIncompleteContracts
		// is the hourly self-heal pass — a fresh DetectContractType probe
		// also picks up NFT contracts we previously couldn't classify
		// (`tokenStandard` migration of legacy rows).
		if !contract.IsToken && contract.Name == "" && contract.Symbol == "" {
			detection, detErr := rpc.DetectContractType(contract.Address)
			if detErr != nil {
				configs.Logger.Debug("Contract type detection failed during reprocess; skipping",
					zap.String("address", contract.Address),
					zap.Error(detErr))
			} else if detection.Standard != "" {
				contract.IsToken = true
				contract.Name = detection.Name
				contract.Symbol = detection.Symbol
				contract.Decimals = detection.Decimals
				contract.TotalSupply = detection.TotalSupply
				contract.TokenStandard = detection.Standard
				contract.HasERC165 = detection.HasERC165
			}
		} else if contract.IsToken && contract.TotalSupply == "" {
			// Get total supply for token with missing supply
			totalSupply, err := rpc.GetTokenTotalSupply(contract.Address)
			if err != nil {
				configs.Logger.Error("Failed to get token total supply",
					zap.String("address", contract.Address),
					zap.Error(err))
			} else {
				contract.TotalSupply = totalSupply
			}
		}

		// Restore original creation information to ensure it's not lost
		// Only restore if the original had values and current values are empty
		if creatorAddress != "" && creatorAddress != "Q" && contract.CreatorAddress == "" {
			contract.CreatorAddress = creatorAddress
		}
		if creationTransaction != "" && contract.CreationTransaction == "" {
			contract.CreationTransaction = creationTransaction
		}
		if creationBlockNumber != "" && contract.CreationBlockNumber == "" {
			contract.CreationBlockNumber = creationBlockNumber
		}

		// Backfill missing creation transaction from the transfer collection
		if contract.CreationTransaction == "" && contract.Address != "" {
			creationTx := findCreationTransaction(contract.Address)
			if creationTx != nil {
				contract.CreationTransaction = creationTx.TxHash
				contract.CreationBlockNumber = creationTx.BlockNumber
				if creationTx.From != "" && creationTx.From != "Q" {
					contract.CreatorAddress = creationTx.From
					configs.Logger.Info("Backfilled creation info from transfer collection",
						zap.String("contract", contract.Address),
						zap.String("creator", contract.CreatorAddress),
						zap.String("tx", contract.CreationTransaction))
				}
			}
		}

		// Backfill missing creator address from creation transaction via RPC
		if (contract.CreatorAddress == "" || contract.CreatorAddress == "Q") && contract.CreationTransaction != "" {
			txDetails, txErr := rpc.GetTxDetailsByHash(contract.CreationTransaction)
			if txErr == nil && txDetails != nil && txDetails.From != "" {
				contract.CreatorAddress = validation.ConvertToQAddress(txDetails.From)
				configs.Logger.Info("Backfilled creator address from creation transaction",
					zap.String("contract", contract.Address),
					zap.String("creator", contract.CreatorAddress))
			}
		}

		contract.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		// Update the contract
		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to update contract",
				zap.String("address", contract.Address),
				zap.Error(err))
			continue
		}

		processedCount++
		if processedCount%100 == 0 {
			configs.Logger.Info("Reprocessing progress",
				zap.Int("processed_contracts", processedCount))
		}
	}

	if err := cursor.Err(); err != nil {
		configs.Logger.Error("Cursor error while reprocessing contracts", zap.Error(err))
		return err
	}

	configs.Logger.Info("Completed reprocessing incomplete contracts",
		zap.Int("total_processed", processedCount))
	return nil
}

// creationTxInfo holds the minimal info needed from a creation transaction
type creationTxInfo struct {
	TxHash      string `bson:"txHash"`
	From        string `bson:"from"`
	BlockNumber string `bson:"blockNumber"`
}

// findCreationTransaction looks up the contract creation transaction.
// It first checks the transfer collection (direct deployments have contractAddress set).
// For factory-deployed contracts it falls back to the tokenTransfers collection,
// finding the initial mint event (from zero address) and resolving the real tx sender.
func findCreationTransaction(contractAddress string) *creationTxInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Direct deployment: transfer collection has contractAddress field
	var result creationTxInfo
	err := configs.TransferCollections.FindOne(ctx, bson.M{
		"contractAddress": contractAddress,
	}).Decode(&result)
	if err == nil {
		return &result
	}

	// 2. Factory deployment: find the initial mint in tokenTransfers
	var mint struct {
		TxHash      string `bson:"txHash"`
		BlockNumber string `bson:"blockNumber"`
	}
	err = configs.GetTokenTransfersCollection().FindOne(ctx, bson.M{
		"contractAddress": contractAddress,
		"from":            "Q0",
	}).Decode(&mint)
	if err != nil || mint.TxHash == "" {
		return nil
	}

	// Look up the actual transaction sender from the transfer collection
	var tx struct {
		From string `bson:"from"`
	}
	err = configs.TransferCollections.FindOne(ctx, bson.M{
		"txHash": mint.TxHash,
	}).Decode(&tx)
	if err != nil {
		// Still return what we have from the mint event
		return &creationTxInfo{
			TxHash:      mint.TxHash,
			BlockNumber: mint.BlockNumber,
		}
	}

	return &creationTxInfo{
		TxHash:      mint.TxHash,
		From:        tx.From,
		BlockNumber: mint.BlockNumber,
	}
}

// StartContractReprocessingJob starts a background job to periodically reprocess incomplete contracts
func StartContractReprocessingJob() {
	go func() {
		for {
			configs.Logger.Info("Starting contract reprocessing job")

			err := ReprocessIncompleteContracts()
			if err != nil {
				configs.Logger.Error("Contract reprocessing job failed", zap.Error(err))
			}

			// Wait for 1 hour before next run
			time.Sleep(1 * time.Hour)
		}
	}()
}
