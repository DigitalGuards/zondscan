package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// ReprocessIncompleteContracts finds and updates contracts with missing information
func ReprocessIncompleteContracts() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find contracts with missing information, including bare "Q" creator
	// addresses. Phase 3a adds an NFT branch: collections classified before
	// the contractURI probe shipped have an empty metadataURI, the
	// fetcher's work-queue filter (metadataURI != "" AND fetchedAt == "")
	// would never include them, so the hourly reprocess pass picks them
	// up here and re-probes contractURI(). $or fans out across the same
	// query, no extra round-trip.
	filter := bson.M{
		"$or": []bson.M{
			{"contractCode": ""},
			{"isToken": true, "totalSupply": ""},
			{"isToken": false, "name": "", "symbol": ""},
			{"creatorAddress": "Q"},
			{"creatorAddress": ""},
			{
				"tokenStandard": bson.M{"$in": []string{"ERC-721", "ERC-1155"}},
				"$or": []bson.M{
					{"metadataURI": bson.M{"$exists": false}},
					{"metadataURI": ""},
				},
			},
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
		// is the hourly self-heal pass, a fresh DetectContractType probe
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

		// Phase 3a backfill: NFT contracts that were classified before the
		// metadataURI probe shipped have an empty MetadataURI. The fetcher
		// service needs a populated URI to enqueue work, so the hourly
		// reprocess pass also re-probes contractURI() for any NFT row
		// missing it. Best-effort: a contract-revert (most collections
		// don't implement contractURI) returns ("", nil) and we leave the
		// field empty; transient failures don't demote existing state.
		if (contract.TokenStandard == rpc.StandardERC721 || contract.TokenStandard == rpc.StandardERC1155) && contract.MetadataURI == "" {
			if uri, err := rpc.GetContractURI(contract.Address); err == nil && uri != "" {
				contract.MetadataURI = uri
				configs.Logger.Info("Backfilled contractURI for NFT collection",
					zap.String("address", contract.Address),
					zap.String("metadataURI", uri))
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

		// Backfill missing creation transaction from the authoritative
		// sources: the transfer collection (direct deploys) and the
		// internal-transaction index (factory deploys via CREATE frames).
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

		// Genesis contracts have no creation transaction anywhere on chain.
		// Probe the code at block 0 BEFORE the mint heuristic below: a
		// genesis-baked token that later emits a mint would otherwise get the
		// mint tx latched as its creation tx, permanently blocking this probe.
		// A hit pins creationBlockNumber to genesis and flags the row so later
		// passes skip the probe. Fail soft on RPC error (the node may lack
		// archive state for historical getCode) and leave the record incomplete.
		if contract.CreationTransaction == "" && contract.CreationBlockNumber == "" && !contract.GenesisContract {
			genesisCode, codeErr := rpc.GetCode(contract.Address, "0x0")
			if codeErr != nil {
				configs.Logger.Debug("Genesis code probe failed; leaving creation info incomplete",
					zap.String("address", contract.Address),
					zap.Error(codeErr))
			} else if genesisCode != "" && genesisCode != "0x" && genesisCode != "0x0" {
				contract.CreationBlockNumber = "0x0"
				contract.GenesisContract = true
				configs.Logger.Info("Contract code present at genesis, pinned creation block to 0x0",
					zap.String("address", contract.Address))
			}
		}

		// Last-resort mint heuristic for factory deploys without internal-tx
		// coverage: the earliest mint is usually the create+mint tx. When the
		// creation block is already known, the mint must sit in that exact
		// block: a later mint would stamp a creation tx from the wrong block.
		if contract.CreationTransaction == "" && !contract.GenesisContract && contract.Address != "" {
			mintTx := findCreationTransactionFromMint(contract.Address, contract.CreationBlockNumber)
			if mintTx != nil {
				contract.CreationTransaction = mintTx.TxHash
				if contract.CreationBlockNumber == "" {
					contract.CreationBlockNumber = mintTx.BlockNumber
				}
				if mintTx.From != "" && mintTx.From != "Q" {
					contract.CreatorAddress = mintTx.From
				}
				configs.Logger.Info("Backfilled creation info from earliest mint",
					zap.String("contract", contract.Address),
					zap.String("tx", contract.CreationTransaction))
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

// findCreationTransaction looks up the contract creation transaction from the
// authoritative sources: the transfer collection (direct deployments have
// contractAddress set), then the internal-transaction index for a
// CREATE/CREATE2 frame targeting the address (authoritative for factory
// deployments, including non-token ones). The mint heuristic lives separately
// in findCreationTransactionFromMint so callers can order the genesis probe
// between the two.
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

	// 2. Factory deployment: CREATE/CREATE2 frame from the internal-transaction
	// index (populated when debug tracing is enabled). An address is only ever
	// created once, so a plain FindOne needs no sort.
	var frame struct {
		Hash        string `bson:"hash"`
		From        string `bson:"from"`
		BlockNumber string `bson:"blockNumber"`
	}
	err = configs.InternalTransactionByAddressCollections.FindOne(ctx, bson.M{
		"type": bson.M{"$in": []string{"CREATE", "CREATE2"}},
		"to":   contractAddress,
	}).Decode(&frame)
	if err == nil && frame.Hash != "" {
		// The frame's own `from` is the immediate caller (the factory
		// contract); resolve the outer transaction sender for the creator
		// field, falling back to the frame's from if the lookup fails.
		from := lookupTransactionSender(ctx, frame.Hash)
		if from == "" {
			from = frame.From
		}
		return &creationTxInfo{
			TxHash:      frame.Hash,
			From:        from,
			BlockNumber: frame.BlockNumber,
		}
	}

	return nil
}

// findCreationTransactionFromMint is the last-resort heuristic for factory
// deployments without internal-tx coverage: find the initial mint in
// tokenTransfers. Mints store the zero address in the full-width spelling
// (StoreTokenTransfer normalizes empty senders to configs.QRLZeroAddress);
// the short "Q0" form is matched too in case any legacy row carries it. Sort
// ascending by blockNumberInt so the EARLIEST mint (the create+mint tx) is
// picked, not an arbitrary one. When the creation block is already known,
// requiredBlock pins the lookup to that block so a later mint cannot be
// misattributed as the creation tx; pass "" when the block is unknown.
func findCreationTransactionFromMint(contractAddress string, requiredBlock string) *creationTxInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"contractAddress": contractAddress,
		"from":            bson.M{"$in": []string{configs.QRLZeroAddress, "Q0"}},
	}
	if requiredBlock != "" {
		filter["blockNumber"] = requiredBlock
	}

	var mint struct {
		TxHash      string `bson:"txHash"`
		BlockNumber string `bson:"blockNumber"`
	}
	mintOpts := options.FindOne().SetSort(bson.D{{Key: "blockNumberInt", Value: 1}})
	err := configs.GetTokenTransfersCollection().FindOne(ctx, filter, mintOpts).Decode(&mint)
	if err != nil || mint.TxHash == "" {
		return nil
	}

	// Look up the actual transaction sender from the transfer collection
	from := lookupTransactionSender(ctx, mint.TxHash)
	return &creationTxInfo{
		TxHash:      mint.TxHash,
		From:        from,
		BlockNumber: mint.BlockNumber,
	}
}

// lookupTransactionSender resolves the outer sender of a transaction from the
// transfer collection. Returns "" when the row is missing; callers treat that
// as "creator unknown" and keep whatever fallback they have.
func lookupTransactionSender(ctx context.Context, txHash string) string {
	var tx struct {
		From string `bson:"from"`
	}
	err := configs.TransferCollections.FindOne(ctx, bson.M{
		"txHash": txHash,
	}).Decode(&tx)
	if err != nil {
		return ""
	}
	return tx.From
}
