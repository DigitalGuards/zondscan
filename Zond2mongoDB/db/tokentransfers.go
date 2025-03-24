package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"Zond2mongoDB/utils"
	"context"
	"fmt"
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
		transfer.From = "0x0" // Normalize empty from address to zero address
	}

	if transfer.To == "" {
		transfer.To = "0x0" // Normalize empty to address to zero address
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

		// Check if this contract is already known to be a token
		contract, err := GetContract(contractAddress)
		if err != nil {
			// Contract not found, check if it's a token
			configs.Logger.Debug("Contract not in database, checking if it's a token",
				zap.String("address", contractAddress))
			name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)

			if isToken {
				configs.Logger.Info("Discovered new token contract",
					zap.String("address", contractAddress),
					zap.String("name", name),
					zap.String("symbol", symbol),
					zap.Uint8("decimals", decimals))

				// Store new token contract
				newContract := models.ContractInfo{
					Address:   contractAddress,
					Status:    "success",
					IsToken:   true,
					Name:      name,
					Symbol:    symbol,
					Decimals:  decimals,
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				}

				err = StoreContract(newContract)
				if err != nil {
					configs.Logger.Error("Failed to store new token contract",
						zap.String("address", contractAddress),
						zap.Error(err))
				}

				contract = &newContract
			} else {
				// Not a token, skip
				configs.Logger.Debug("Contract is not a token, skipping",
					zap.String("address", contractAddress))
				continue
			}
		} else if !contract.IsToken {
			// Known contract but not a token, skip
			configs.Logger.Debug("Known contract but not a token, skipping",
				zap.String("address", contractAddress))
			continue
		}

		// Extract from and to addresses
		from := "0x" + rpc.TrimLeftZeros(log.Topics[1][26:])
		to := "0x" + rpc.TrimLeftZeros(log.Topics[2][26:])

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
		if from == "" {
			from = "0x0"
		}

		if to == "" {
			to = "0x0"
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

// DetectFactoryCreatedTokens searches for token creation events in a block range
func DetectFactoryCreatedTokens(fromBlock, toBlock string) error {
	configs.Logger.Info("Scanning for factory-created tokens",
		zap.String("fromBlock", fromBlock),
		zap.String("toBlock", toBlock))

	// List of token creation event signatures to scan for
	eventSignatures := []string{
		rpc.TokenDeployedEventSignature1,
		rpc.TokenDeployedEventSignature2,
		rpc.PairCreatedEventSignature,
	}

	// Also scan for direct token creation method calls
	txMethodSignatures := []string{
		rpc.TokenCreationSignature,
	}

	tokensDiscovered := 0

	// Process each event signature
	for _, eventSig := range eventSignatures {
		configs.Logger.Info("Scanning for event signature",
			zap.String("signature", eventSig))

		// Query logs with this event signature
		logs, err := rpc.ZondGetLogs(fromBlock, toBlock, nil, []string{eventSig})
		if err != nil {
			configs.Logger.Error("Failed to get logs for event signature",
				zap.String("signature", eventSig),
				zap.Error(err))
			continue
		}

		if logs == nil || len(logs.Result) == 0 {
			configs.Logger.Debug("No logs found for event signature",
				zap.String("signature", eventSig))
			continue
		}

		configs.Logger.Info("Found potential token creation events",
			zap.String("signature", eventSig),
			zap.Int("count", len(logs.Result)))

		// Process each log
		for _, logEntry := range logs.Result {
			// Different processing based on event signature
			var tokenAddress string
			var factoryAddress string

			// Extract token address from the log
			if eventSig == rpc.TokenDeployedEventSignature1 || eventSig == rpc.TokenDeployedEventSignature2 {
				// For TokenDeployed events, token address is usually in topics[1]
				if len(logEntry.Topics) >= 2 {
					// Extract address, removing leading zeros
					tokenAddress = "Z" + rpc.TrimLeftZeros(logEntry.Topics[1][26:])
				}
				factoryAddress = logEntry.Address
			} else if eventSig == rpc.PairCreatedEventSignature {
				// For PairCreated events, pair address is usually in topics[1] or topics[2]
				if len(logEntry.Topics) >= 3 {
					tokenAddress = "Z" + rpc.TrimLeftZeros(logEntry.Topics[2][26:])
				}
				factoryAddress = logEntry.Address
			}

			// Skip if we couldn't determine the token address
			if tokenAddress == "" {
				configs.Logger.Debug("Could not extract token address from log",
					zap.String("txHash", logEntry.TransactionHash))
				continue
			}

			// Process the token
			tokensFound := processTokenAddress(tokenAddress, factoryAddress, logEntry.TransactionHash)
			tokensDiscovered += tokensFound
		}
	}

	// Scan blocks for direct token creation method calls
	// Instead of processing each block individually, query for contract creation transactions in batches
	configs.Logger.Info("Scanning for direct token creation transactions - optimized approach")

	// Convert block range to integers for batch calculation
	fromBlockInt := utils.HexToInt(fromBlock).Int64()
	toBlockInt := utils.HexToInt(toBlock).Int64()

	// Use larger batch size for faster processing
	batchSize := int64(100) // Process 100 blocks at a time

	// Process in batches
	for batchStart := fromBlockInt; batchStart <= toBlockInt; batchStart += batchSize {
		batchEnd := batchStart + batchSize - 1
		if batchEnd > toBlockInt {
			batchEnd = toBlockInt
		}

		// Convert to hex strings directly
		batchStartHex := fmt.Sprintf("0x%x", batchStart)
		batchEndHex := fmt.Sprintf("0x%x", batchEnd)

		configs.Logger.Info("Processing contract creation batch",
			zap.String("fromBlock", batchStartHex),
			zap.String("toBlock", batchEndHex))

		// Get contract creation transactions in this batch
		// This approach looks for transactions with empty 'to' field (contract creation)
		creationTxs, err := findContractCreationTransactions(batchStartHex, batchEndHex)
		if err != nil {
			configs.Logger.Error("Failed to find contract creation transactions",
				zap.String("fromBlock", batchStartHex),
				zap.String("toBlock", batchEndHex),
				zap.Error(err))
			continue
		}

		// Process each transaction
		for _, tx := range creationTxs {
			// Check if this transaction uses our method signature
			for _, methodSig := range txMethodSignatures {
				if len(tx.Data) >= 10 && strings.HasPrefix(tx.Data, methodSig) {
					// Get receipt to find created contract address
					receipt, err := rpc.GetTransactionReceipt(tx.Hash)
					if err != nil {
						configs.Logger.Error("Failed to get receipt for token creation tx",
							zap.String("txHash", tx.Hash),
							zap.Error(err))
						continue
					}

					if receipt != nil && receipt.Result.ContractAddress != "" {
						tokenAddress := receipt.Result.ContractAddress
						factoryAddress := tx.From

						configs.Logger.Info("Found contract created from token creation method",
							zap.String("tokenAddress", tokenAddress),
							zap.String("factoryAddress", factoryAddress),
							zap.String("txHash", tx.Hash))

						// Process the token
						tokensFound := processTokenAddress(tokenAddress, factoryAddress, tx.Hash)
						tokensDiscovered += tokensFound
					}
				}
			}
		}

		// Add a small delay between batches to prevent overwhelming the node
		time.Sleep(50 * time.Millisecond)
	}

	configs.Logger.Info("Factory token detection completed",
		zap.String("fromBlock", fromBlock),
		zap.String("toBlock", toBlock),
		zap.Int("tokensDiscovered", tokensDiscovered))

	return nil
}

// findContractCreationTransactions returns transactions that create contracts in a block range
// These are transactions with empty 'to' field
func findContractCreationTransactions(fromBlock, toBlock string) ([]models.Transaction, error) {
	var creationTxs []models.Transaction

	// Query MongoDB for transactions in this block range that have empty 'to' field
	collection := configs.GetCollection(configs.DB, "transfers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"blockNumber": bson.M{
			"$gte": fromBlock,
			"$lte": toBlock,
		},
		"to": "",
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Process results
	for cursor.Next(ctx) {
		var tx models.Transaction
		if err := cursor.Decode(&tx); err != nil {
			configs.Logger.Error("Failed to decode transaction",
				zap.Error(err))
			continue
		}
		creationTxs = append(creationTxs, tx)
	}

	return creationTxs, nil
}

// processTokenAddress checks if an address is a token and stores it if it is
// Returns 1 if a token was discovered and stored, 0 otherwise
func processTokenAddress(tokenAddress string, factoryAddress string, txHash string) int {
	configs.Logger.Info("Processing potential factory-created token",
		zap.String("tokenAddress", tokenAddress),
		zap.String("factoryAddress", factoryAddress),
		zap.String("txHash", txHash))

	// Check if the contract is already known
	contract, err := GetContract(tokenAddress)
	if err == nil && contract != nil {
		// If we already know about this token but didn't mark it as factory-created,
		// update it with factory information
		if !contract.CreatedByFactory {
			configs.Logger.Info("Updating existing token with factory information",
				zap.String("address", tokenAddress),
				zap.String("factory", factoryAddress))

			contract.CreatedByFactory = true
			contract.FactoryAddress = factoryAddress
			StoreContract(*contract)
		}
		return 0
	}

	// Check if this contract is a token
	name, symbol, decimals, isToken := rpc.GetTokenInfo(tokenAddress)
	if !isToken {
		configs.Logger.Debug("Contract is not a token",
			zap.String("address", tokenAddress))
		return 0
	}

	configs.Logger.Info("Discovered factory-created token",
		zap.String("address", tokenAddress),
		zap.String("name", name),
		zap.String("symbol", symbol),
		zap.Uint8("decimals", decimals),
		zap.String("factory", factoryAddress))

	// Get total supply
	totalSupply, _ := rpc.GetTokenTotalSupply(tokenAddress)

	// Get custom token info
	customInfo, _ := rpc.GetCustomTokenInfo(tokenAddress)

	// Store the token contract
	newContract := models.ContractInfo{
		Address:             tokenAddress,
		Status:              "success",
		IsToken:             true,
		Name:                name,
		Symbol:              symbol,
		Decimals:            decimals,
		TotalSupply:         totalSupply,
		CreatedByFactory:    true,
		FactoryAddress:      factoryAddress,
		CreationTransaction: txHash,
		UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
	}

	// Add custom token properties if available
	if val, ok := customInfo["maxSupply"]; ok {
		newContract.MaxSupply = val
	}
	if val, ok := customInfo["maxTxLimit"]; ok {
		newContract.MaxTxLimit = val
	}
	if val, ok := customInfo["maxWalletAmount"]; ok {
		newContract.MaxWalletAmount = val
	}
	if val, ok := customInfo["tokenOwner"]; ok {
		newContract.TokenOwner = val
	}

	// Store the contract
	if err := StoreContract(newContract); err != nil {
		configs.Logger.Error("Failed to store token contract",
			zap.String("address", tokenAddress),
			zap.Error(err))
		return 0
	}

	return 1
}
