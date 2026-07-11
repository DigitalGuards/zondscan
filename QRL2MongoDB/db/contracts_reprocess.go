package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
