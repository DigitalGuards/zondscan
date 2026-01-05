package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"context"
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
	ctx := context.Background()

	// Log the collection name
	configs.Logger.Info("Using collection for token transfers",
		zap.String("collection", "tokenTransfers"))

	// Note: indexes are created once during initialization in InitializeTokenTransfersCollection()
	// We don't create indexes here to avoid "IndexKeySpecsConflict" errors on high-frequency calls

	// Store the transfer
	configs.Logger.Info("Inserting token transfer document",
		zap.String("token", transfer.TokenSymbol),
		zap.String("from", transfer.From),
		zap.String("to", transfer.To),
		zap.String("txHash", transfer.TxHash))

	// Additional validation and normalization before inserting
	if transfer.From == "" {
		transfer.From = "Z0000000000000000000000000000000000000000" // Normalize empty from address to zero address
	}

	if transfer.To == "" {
		transfer.To = "Z0000000000000000000000000000000000000000" // Normalize empty to address to zero address
	}

	_, err := collection.InsertOne(ctx, transfer)
	if err != nil {
		configs.Logger.Error("Failed to store token transfer",
			zap.String("txHash", transfer.TxHash),
			zap.String("token", transfer.TokenSymbol),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully stored token transfer in database",
		zap.String("token", transfer.TokenSymbol),
		zap.String("txHash", transfer.TxHash))
	return nil
}

// GetTokenTransfersByContract retrieves all transfers for a specific token contract
func GetTokenTransfersByContract(contractAddress string, skip, limit int64) ([]models.TokenTransfer, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")
	ctx := context.Background()

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
	ctx := context.Background()

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

// TokenTransferExists checks if a token transfer already exists in the database
func TokenTransferExists(txHash string, contractAddress string, from string, to string) (bool, error) {
	collection := configs.GetCollection(configs.DB, "tokenTransfers")
	ctx := context.Background()

	filter := bson.M{
		"txHash":          txHash,
		"contractAddress": contractAddress,
		"from":            from,
		"to":              to,
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		configs.Logger.Error("Failed to check if token transfer exists",
			zap.String("txHash", txHash),
			zap.Error(err))
		return false, err
	}

	return count > 0, nil
}

// ProcessBlockTokenTransfers processes all token transfers in a block
func ProcessBlockTokenTransfers(blockNumber string, blockTimestamp string) error {
	// Get logs for the Transfer event signature
	transferEventSignature := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	configs.Logger.Info("Searching for token transfers",
		zap.String("blockNumber", blockNumber),
		zap.String("eventSignature", transferEventSignature))

	response, err := rpc.ZondGetBlockLogs(blockNumber, []string{transferEventSignature})
	if err != nil {
		configs.Logger.Error("Failed to get logs for block",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return err
	}

	if response == nil || len(response.Result) == 0 {
		configs.Logger.Debug("No token transfer logs found in block",
			zap.String("blockNumber", blockNumber))
		return nil // No logs found
	}

	configs.Logger.Info("Found potential token transfer logs",
		zap.String("blockNumber", blockNumber),
		zap.Int("logCount", len(response.Result)))

	// Process each log
	tokenTransfersFound := 0
	for _, log := range response.Result {
		// Skip logs with insufficient topics
		if len(log.Topics) < 3 {
			configs.Logger.Debug("Skipping log with insufficient topics",
				zap.String("txHash", log.TransactionHash),
				zap.Int("topicCount", len(log.Topics)))
			continue
		}

		// Extract contract address
		contractAddress := log.Address
		configs.Logger.Debug("Processing potential token transfer",
			zap.String("contractAddress", contractAddress),
			zap.String("txHash", log.TransactionHash))

		// ---> Double-check token status via RPC before checking DB <---
		name, symbol, decimals, isTokenRPC := rpc.GetTokenInfo(contractAddress)
		if isTokenRPC {
			configs.Logger.Debug("RPC check confirms contract is a token (or potentially is)",
				zap.String("address", contractAddress))

			// First try to get existing contract from DB to preserve creation information
			existingContract, err := GetContract(contractAddress)

			// Prepare contract info based on RPC result
			contractUpdate := models.ContractInfo{
				Address:   contractAddress,
				Status:    "0x1", // Assume successful
				IsToken:   true,
				Name:      name,
				Symbol:    symbol,
				Decimals:  decimals,
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				// Add creation information if this is a new token discovery
				CreationBlockNumber: blockNumber,         // Set the block where we found the token
				CreationTransaction: log.TransactionHash, // Set the transaction that contained the transfer
			}

			// Try to get transaction details to find the creator
			txDetails, txErr := rpc.GetTxDetailsByHash(log.TransactionHash)
			if txErr == nil && txDetails != nil {
				configs.Logger.Debug("Retrieved transaction details for token creator identification",
					zap.String("txHash", log.TransactionHash))
				contractUpdate.CreatorAddress = txDetails.From
			} else {
				configs.Logger.Debug("Failed to get transaction details for token discovery",
					zap.String("txHash", log.TransactionHash),
					zap.Error(txErr))
			}

			// If we have existing contract data, preserve the existing creation information
			// as it might be more accurate than what we just discovered
			if err == nil && existingContract != nil {
				// Preserve creation information if present
				if existingContract.CreatorAddress != "" {
					contractUpdate.CreatorAddress = existingContract.CreatorAddress
				}
				if existingContract.CreationTransaction != "" {
					contractUpdate.CreationTransaction = existingContract.CreationTransaction
				}
				if existingContract.CreationBlockNumber != "" {
					contractUpdate.CreationBlockNumber = existingContract.CreationBlockNumber
				}
				if existingContract.ContractCode != "" {
					contractUpdate.ContractCode = existingContract.ContractCode
				}
			}

			// Attempt to get total supply as well
			totalSupply, supplyErr := rpc.GetTokenTotalSupply(contractAddress)
			if supplyErr == nil {
				contractUpdate.TotalSupply = totalSupply
			} else {
				configs.Logger.Debug("Failed to get total supply during RPC double-check",
					zap.String("address", contractAddress),
					zap.Error(supplyErr))
			}

			// Store/Merge this information into the database
			errUpdate := StoreContract(contractUpdate)
			if errUpdate != nil {
				configs.Logger.Error("Failed to store/update contract during RPC double-check",
					zap.String("address", contractAddress),
					zap.Error(errUpdate))
				// Continue processing the log anyway, maybe DB check will work
			}
		} else {
			configs.Logger.Debug("RPC check indicates contract is not a token",
				zap.String("address", contractAddress))
			// Optionally: Update DB to set IsToken=false if desired, but might conflict with other logic.
			// For now, just rely on the subsequent DB check.
		}
		// ---> End of double-check <---

		// Check if this contract is already known to be a token in DB (might have been updated above)
		contract, err := GetContract(contractAddress)
		if err != nil {
			// If GetContract failed even after potentially storing it, log error and skip.
			// This case covers the original 'contract not found' scenario, but now after an attempted update.
			// If isTokenRPC was true but StoreContract failed, we log and skip here.
			// If isTokenRPC was false, this follows the original 'contract not found and not a token' path.
			configs.Logger.Warn("Failed to get contract from DB after RPC check, skipping log",
				zap.String("address", contractAddress),
				zap.Error(err),
				zap.Bool("isTokenRPC", isTokenRPC))
			continue
		}

		// Now, check the IsToken status from the potentially updated DB record
		if !contract.IsToken {
			// Known contract in DB, but confirmed not a token (either originally or by RPC double-check)
			configs.Logger.Debug("Contract confirmed as not a token in DB, skipping",
				zap.String("address", contractAddress))
			continue
		}

		// Extract from and to addresses (use Z prefix for QRL addresses)
		from := "Z" + rpc.TrimLeftZeros(log.Topics[1][26:])
		to := "Z" + rpc.TrimLeftZeros(log.Topics[2][26:])

		configs.Logger.Debug("Token transfer details",
			zap.String("from", from),
			zap.String("to", to),
			zap.String("token", contract.Symbol))

		// Extract amount
		amount := log.Data

		// Check if this transfer already exists
		exists, err := TokenTransferExists(log.TransactionHash, contractAddress, from, to)
		if err != nil {
			configs.Logger.Error("Failed to check if token transfer exists",
				zap.String("txHash", log.TransactionHash),
				zap.Error(err))
			continue
		}

		if exists {
			// Skip duplicate transfers
			configs.Logger.Debug("Skipping duplicate token transfer",
				zap.String("txHash", log.TransactionHash))
			continue
		}

		// Normalize addresses to ensure consistency
		if from == "" || from == "Z" {
			from = "Z0000000000000000000000000000000000000000"
		}

		if to == "" || to == "Z" {
			to = "Z0000000000000000000000000000000000000000"
		}

		// Log token transfer identified
		configs.Logger.Info("Identified token transfer",
			zap.String("token", contract.Symbol),
			zap.String("from", from),
			zap.String("to", to),
			zap.String("amount", amount),
			zap.String("blockNumber", blockNumber))

		// Create token transfer record
		transfer := models.TokenTransfer{
			ContractAddress: contractAddress,
			From:            from,
			To:              to,
			Amount:          amount,
			BlockNumber:     blockNumber,
			TxHash:          log.TransactionHash,
			Timestamp:       blockTimestamp,
			TokenSymbol:     contract.Symbol,
			TokenDecimals:   contract.Decimals,
			TokenName:       contract.Name,
			TransferType:    "event",
		}

		// Store the transfer
		err = StoreTokenTransfer(transfer)
		if err != nil {
			configs.Logger.Error("Failed to store token transfer",
				zap.String("txHash", log.TransactionHash),
				zap.Error(err))
			continue
		} else {
			tokenTransfersFound++
			configs.Logger.Info("Successfully stored token transfer",
				zap.String("txHash", log.TransactionHash),
				zap.String("token", contract.Symbol),
				zap.String("from", from),
				zap.String("to", to))
		}

		// Update token balances
		configs.Logger.Info("Attempting to update token balances for transfer",
			zap.String("txHash", log.TransactionHash),
			zap.String("contractAddress", contractAddress),
			zap.String("from", from),
			zap.String("to", to),
			zap.String("amount", amount))

		err = StoreTokenBalance(contractAddress, from, amount, blockNumber)
		if err != nil {
			configs.Logger.Error("Failed to update sender token balance",
				zap.String("address", from),
				zap.String("contractAddress", contractAddress),
				zap.Error(err))
		} else {
			configs.Logger.Info("Successfully updated sender token balance",
				zap.String("address", from),
				zap.String("contractAddress", contractAddress))
		}

		err = StoreTokenBalance(contractAddress, to, amount, blockNumber)
		if err != nil {
			configs.Logger.Error("Failed to update recipient token balance",
				zap.String("address", to),
				zap.String("contractAddress", contractAddress),
				zap.Error(err))
		} else {
			configs.Logger.Info("Successfully updated recipient token balance",
				zap.String("address", to),
				zap.String("contractAddress", contractAddress))
		}
	}

	configs.Logger.Info("Finished processing token transfers",
		zap.String("blockNumber", blockNumber),
		zap.Int("transfersProcessed", tokenTransfersFound))

	return nil
}

// InitializeTokenTransfersCollection ensures the token transfers collection is set up with proper indexes
func InitializeTokenTransfersCollection() error {
	collection := configs.GetTokenTransfersCollection()
	ctx := context.Background()

	configs.Logger.Info("Initializing tokenTransfers collection and indexes")

	// Create indexes for token transfers collection
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
			Keys:    bson.D{{Key: "txHash", Value: 1}},
			Options: options.Index().SetName("txHash_idx").SetUnique(true),
		},
	}

	// First drop any existing indexes to avoid conflicts
	_, err := collection.Indexes().DropAll(ctx)
	if err != nil {
		configs.Logger.Warn("Failed to drop existing indexes, attempting to continue",
			zap.Error(err))
	}

	// Create the new indexes
	_, err = collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for token transfers",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized tokenTransfers collection and indexes")
	return nil
}

// InitializeTokenBalancesCollection ensures the token balances collection is set up with proper indexes
func InitializeTokenBalancesCollection() error {
	collection := configs.GetTokenBalancesCollection()
	ctx := context.Background()

	configs.Logger.Info("Initializing tokenBalances collection and indexes")

	// Create indexes for token balances collection
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "address", Value: 1},
			},
			Options: options.Index().SetName("contract_address_idx").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "address", Value: 1},
			},
			Options: options.Index().SetName("address_idx"),
		},
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
			},
			Options: options.Index().SetName("contract_idx"),
		},
	}

	// First drop any existing indexes to avoid conflicts
	_, err := collection.Indexes().DropAll(ctx)
	if err != nil {
		configs.Logger.Warn("Failed to drop existing indexes for token balances, attempting to continue",
			zap.Error(err))
	}

	// Create the new indexes
	_, err = collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for token balances",
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully initialized tokenBalances collection and indexes")
	return nil
}
