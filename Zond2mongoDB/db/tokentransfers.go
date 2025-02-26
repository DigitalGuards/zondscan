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
	collection := configs.GetCollection(configs.DB, "tokenTransfers")
	ctx := context.Background()

	// Create indexes if they don't exist
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "contractAddress", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "from", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
				{Key: "blockNumber", Value: 1},
			},
		},
		{
			Keys:    bson.D{{Key: "txHash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		configs.Logger.Error("Failed to create indexes for token transfers",
			zap.Error(err))
	}

	// Store the transfer
	_, err = collection.InsertOne(ctx, transfer)
	if err != nil {
		configs.Logger.Error("Failed to store token transfer",
			zap.String("txHash", transfer.TxHash),
			zap.Error(err))
		return err
	}

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

	response, err := rpc.ZondGetBlockLogs(blockNumber, []string{transferEventSignature})
	if err != nil {
		configs.Logger.Error("Failed to get logs for block",
			zap.String("blockNumber", blockNumber),
			zap.Error(err))
		return err
	}

	if response == nil || len(response.Result) == 0 {
		return nil // No logs found
	}

	// Process each log
	for _, log := range response.Result {
		// Skip logs with insufficient topics
		if len(log.Topics) < 3 {
			continue
		}

		// Extract contract address
		contractAddress := log.Address

		// Check if this contract is already known to be a token
		contract, err := GetContract(contractAddress)
		if err != nil {
			// Contract not found, check if it's a token
			name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)
			if isToken {
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
				continue
			}
		} else if !contract.IsToken {
			// Known contract but not a token, skip
			continue
		}

		// Extract from and to addresses
		from := "0x" + rpc.TrimLeftZeros(log.Topics[1][26:])
		to := "0x" + rpc.TrimLeftZeros(log.Topics[2][26:])

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
			continue
		}

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
		}

		// Update token balances
		err = StoreTokenBalance(contractAddress, from, amount, blockNumber)
		if err != nil {
			configs.Logger.Error("Failed to update sender token balance",
				zap.String("address", from),
				zap.Error(err))
		}

		err = StoreTokenBalance(contractAddress, to, amount, blockNumber)
		if err != nil {
			configs.Logger.Error("Failed to update recipient token balance",
				zap.String("address", to),
				zap.Error(err))
		}
	}

	return nil
}
